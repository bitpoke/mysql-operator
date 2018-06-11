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

package clustercontroller

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/wgliang/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myclientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
)

var (
	lockJobRegister sync.Mutex
	// polling time for backup to be completed
	backupPollingTime = time.Second
	// time to wait for a backup to be completed
	backupWatchTimeout = time.Hour
)

// The job structure contains the context to schedule a backup
type job struct {
	Name      string
	Namespace string

	BackupRunning *bool

	lock     *sync.Mutex
	myClient myclientset.Interface
}

func (c *Controller) registerClusterInBackupCron(cluster *api.MysqlCluster) error {
	glog.Infof("Register cluster into cronjob: %s, crontab: %s",
		cluster.Name, cluster.Spec.BackupSchedule)

	if len(cluster.Spec.BackupSchedule) == 0 {
		return nil
	}

	schedule, err := cron.Parse(cluster.Spec.BackupSchedule)
	if err != nil {
		return fmt.Errorf("failed to parse schedule: %s", err)
	}

	lockJobRegister.Lock()
	defer lockJobRegister.Unlock()

	for _, entry := range c.cron.Entries() {
		j, ok := entry.Job.(job)
		if ok && j.Name == cluster.Name && j.Namespace == cluster.Namespace {
			glog.V(3).Infof("Cluster %s already added to cron.", cluster.Name)
			if !reflect.DeepEqual(entry.Schedule, schedule) {
				glog.Infof("Update cluster '%s' scheduler to: %s",
					cluster.Name, cluster.Spec.BackupSchedule)
				c.cron.Remove(cluster.Name)
				break
			}
			return nil
		}
	}

	c.cron.Schedule(schedule, job{
		Name:          cluster.Name,
		Namespace:     cluster.Namespace,
		myClient:      c.myClient,
		BackupRunning: new(bool),
		lock:          new(sync.Mutex),
	}, cluster.Name)

	return nil
}

func (j job) Run() {
	backupName := fmt.Sprintf("%s-auto-backup-%s", j.Name, time.Now().Format("2006-01-02t15-04-05"))
	glog.Infof("Schedul backup job started. Creating backup %s..", backupName)

	// Wrap backup creation to ensure that lock is released when backup is
	// created
	created := func() bool {
		j.lock.Lock()
		defer j.lock.Unlock()

		if *j.BackupRunning {
			glog.Infof("Last scheduled backup still running! Can't initiate a new backup for cluster: %s",
				j.Name)
			return false
		}

		tries := 0
		for {
			var err error
			_, err = j.myClient.Mysql().MysqlBackups(j.Namespace).Create(&api.MysqlBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name: backupName,
					Labels: map[string]string{
						"recurrent": "true",
					},
				},
				Spec: api.BackupSpec{
					ClusterName: j.Name,
				},
			})

			if err == nil {
				break
			}
			glog.V(3).Infof("Fail to create backup %s, err: %s", backupName, err)

			if tries > 5 {
				glog.Errorf("Fail to create backup for cluster: %s, err: %s, max tries %d exeded!",
					j.Name, err, tries)
				return false
			}

			time.Sleep(5 * time.Second)
			tries += 1
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
		b, err := j.myClient.Mysql().MysqlBackups(j.Namespace).Get(backupName, metav1.GetOptions{})
		if err != nil {
			glog.Warningf("Failed to get backup %s, err %s", backupName, err)
			return false, nil
		}
		if b.Status.Completed {
			glog.Infof("Backup '%s' finished.", backupName)
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		glog.Errorf("Waiting for backup '%s' for cluster '%s' to be ready failed: %s",
			backupName, j.Name, err)
	}
}
