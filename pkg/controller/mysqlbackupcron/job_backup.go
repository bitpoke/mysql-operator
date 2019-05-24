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
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// The job structure contains the context to schedule a backup
type job struct {
	ClusterName string
	Namespace   string

	// kubernetes client
	c client.Client

	BackupScheduleJobsHistoryLimit *int
	BackupRemoteDeletePolicy       api.DeletePolicy
}

func (j *job) Run() {
	log.Info("scheduled backup job started", "namespace", j.Namespace, "cluster_name", j.ClusterName)

	// run garbage collector if needed
	if j.BackupScheduleJobsHistoryLimit != nil {
		defer j.backupGC()
	}

	// check if a backup is running
	if j.scheduledBackupsRunningCount() > 0 {
		log.V(1).Info("at least a backup is running",
			"backups_len", j.scheduledBackupsRunningCount())
		return
	}

	// create the backup
	if _, err := j.createBackup(); err != nil {
		log.Error(err, "failed to create backup")
	}
}

func (j *job) scheduledBackupsRunningCount() int {
	backupsList := &api.MysqlBackupList{}
	// select all backups with labels recurrent=true and and not completed of the cluster
	selector := j.backupSelector()
	selector.MatchingField("status.completed", "false")

	if err := j.c.List(context.TODO(), selector, backupsList); err != nil {
		log.Error(err, "failed getting backups", "selector", selector)
		return 0
	}

	return len(backupsList.Items)
}

func (j *job) createBackup() (*api.MysqlBackup, error) {
	backupName := fmt.Sprintf("%s-auto-%s", j.ClusterName, time.Now().Format("2006-01-02t15-04-05"))

	backup := &api.MysqlBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: j.Namespace,
			Labels:    j.recurrentBackupLabels(),
		},
		Spec: api.MysqlBackupSpec{
			ClusterName:        j.ClusterName,
			RemoteDeletePolicy: j.BackupRemoteDeletePolicy,
		},
	}
	return backup, j.c.Create(context.TODO(), backup)
}

func (j *job) backupSelector() *client.ListOptions {
	return client.InNamespace(j.Namespace).MatchingLabels(j.recurrentBackupLabels())
}

func (j *job) recurrentBackupLabels() map[string]string {
	return map[string]string{
		"recurrent": "true",
		"cluster":   j.ClusterName,
	}
}

func (j *job) backupGC() {
	var err error
	backupsList := &api.MysqlBackupList{}
	if err = j.c.List(context.TODO(), j.backupSelector(), backupsList); err != nil {
		log.Error(err, "failed getting backups", "selector", j.backupSelector())
		return
	}

	// sort backups by creation time before removing extra backups
	sort.Sort(byTimestamp(backupsList.Items))

	for i, backup := range backupsList.Items {
		if i >= *j.BackupScheduleJobsHistoryLimit {
			// delete the backup
			if err = j.c.Delete(context.TODO(), &backup); err != nil {
				log.Error(err, "failed to delete a backup", "backup", backup)
			}
		}
	}
}

type byTimestamp []api.MysqlBackup

func (a byTimestamp) Len() int      { return len(a) }
func (a byTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byTimestamp) Less(i, j int) bool {
	return a[j].ObjectMeta.CreationTimestamp.Before(&a[i].ObjectMeta.CreationTimestamp)
}
