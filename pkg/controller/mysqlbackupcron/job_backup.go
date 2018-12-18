/*
Copyright 2018 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysqlbackupcron

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var (
	// polling time for backup to be completed
	backupPollingTime = 5 * time.Second
	// time to wait for a backup to be completed
	backupWatchTimeout = time.Hour
)

// The job structure contains the context to schedule a backup
type job struct {
	Name      string
	Namespace string

	BackupRunning *bool

	lock *sync.Mutex
	c    client.Client

	BackupScheduleJobsHistoryLimit *int
}

func (j job) Run() {
	backupName := fmt.Sprintf("%s-auto-backup-%s", j.Name, time.Now().Format("2006-01-02t15-04-05"))
	backupKey := types.NamespacedName{Name: backupName, Namespace: j.Namespace}
	log.Info("scheduled backup job started", "namespace", j.Namespace, "name", backupName)

	if j.BackupScheduleJobsHistoryLimit != nil {
		defer j.backupGC()
	}

	// Wrap backup creation to ensure that lock is released when backup is
	// created

	created := func() bool {
		j.lock.Lock()
		defer j.lock.Unlock()

		if *j.BackupRunning {
			log.Info("last scheduled backup still running! Can't initiate a new backup",
				"cluster", fmt.Sprintf("%s/%s", j.Namespace, j.Name))
			return false
		}

		tries := 0
		for {
			var err error
			cluster := &api.MysqlBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: j.Namespace,
					Labels: map[string]string{
						"recurrent": "true",
					},
				},
				Spec: api.MysqlBackupSpec{
					ClusterName: j.Name,
				},
			}
			if err = j.c.Create(context.TODO(), cluster); err == nil {
				break
			}

			if tries > 5 {
				log.Error(err, "fail to create backup, max tries exeded",
					"cluster", j.Name, "retries", tries, "backup", backupName)
				return false
			}

			log.Info("failed to create backup, retring", "backup", backupName,
				"error", err, "tries", tries)

			time.Sleep(5 * time.Second)
			tries++
		}

		*j.BackupRunning = true
		return true
	}()
	if !created {
		return
	}

	defer func() {
		j.lock.Lock()
		defer j.lock.Unlock()
		*j.BackupRunning = false
	}()

	err := wait.PollImmediate(backupPollingTime, backupWatchTimeout, func() (bool, error) {
		backup := &api.MysqlBackup{}
		if err := j.c.Get(context.TODO(), backupKey, backup); err != nil {
			log.Info("failed to get backup", "backup", backupName, "error", err)
			return false, nil
		}
		if backup.Status.Completed {
			log.Info("backup finished", "backup", backup)
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		log.Error(err, "waiting for backup to finish, failed",
			"backup", backupName, "cluster", fmt.Sprintf("%s/%s", j.Namespace, j.Name))
	}
}

func (j *job) backupGC() {
	var err error

	backupsList := &api.MysqlBackupList{}
	selector := &client.ListOptions{}
	selector = selector.InNamespace(j.Namespace).MatchingLabels(map[string]string{"recurrent": "true"})

	if err = j.c.List(context.TODO(), selector, backupsList); err != nil {
		log.Error(err, "failed getting backups", "selector", selector)
		return
	}

	for i, backup := range backupsList.Items {
		if i > *j.BackupScheduleJobsHistoryLimit {
			// delete the backup
			if err = j.c.Delete(context.TODO(), &backup); err != nil {
				log.Error(err, "failed to delete a backup", "backup", backup)
			}
		}
	}

}
