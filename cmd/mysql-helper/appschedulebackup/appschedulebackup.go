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

package appschedulebackup

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myClientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
)

func RunCommand(stopCh <-chan struct{}, namespace, cluster string) error {
	glog.Infof("Schedule backup for cluster: %s", cluster)
	myClient, err := getTitaniumClient()
	if err != nil {
		return fmt.Errorf("kube client config: %s", err)
	}

	backup, err := createBackup(myClient, namespace, cluster)
	if err != nil {
		return fmt.Errorf("create backup: %s", err)
	}

	// Wait for backup to finish. Waiting for it guarantees that two
	// backups will not overlap.

	for {
		select {
		case <-stopCh:
			break
		case <-time.After(time.Second):
			b, err := myClient.Mysql().MysqlBackups(namespace).Get(backup.Name, meta.GetOptions{})
			if err != nil {
				glog.Warningf("Failed to get backup: %s", err)
			}
			if i, ok := backupConditionIndex(api.BackupComplete, b.Status.Conditions); ok {
				cond := b.Status.Conditions[i]
				if cond.Status == core.ConditionTrue {
					glog.Infof("Backup '%s' finished.", backup.Name)
					return nil
				}
			}
		case <-time.After(time.Hour): // TODO: make duration constant
			return fmt.Errorf("timeout occured while waiting for backup: %s", backup.Name)
		}
	}

	return nil
}

func createBackup(myClient myClientset.Interface, ns, cluster string) (*api.MysqlBackup, error) {
	now := time.Now().Format("2006-01-02t15-04-05")
	return myClient.Mysql().MysqlBackups(ns).Create(&api.MysqlBackup{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%s-auto-backup-%s", cluster, now),
			Labels: map[string]string{
				"recurrent": "true",
			},
		},
		Spec: api.BackupSpec{
			ClusterName: cluster,
		},
	})
}

func getTitaniumClient() (myClientset.Interface, error) {
	// Load the users Kubernetes config
	// in cluster use
	kubeCfg, err := kube.KubeConfig("")

	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %s", err.Error())
	}

	// Create a Navigator api client
	myClient, err := myClientset.NewForConfig(kubeCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating internal group client: %s", err.Error())
	}

	return myClient, nil
}

func backupConditionIndex(ty api.BackupConditionType, cs []api.BackupCondition) (int, bool) {
	for i, cond := range cs {
		if cond.Type == ty {
			return i, true
		}
	}
	return 0, false
}
