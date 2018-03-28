package appschedulebackup

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	tiClientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	"github.com/presslabs/mysql-operator/pkg/util"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
)

func RunCommand(stopCh <-chan struct{}, namespace, cluster string) error {
	glog.Infof("Schedule backup for cluster: %s", cluster)
	tiClient, err := getTitaniumClient()
	if err != nil {
		return fmt.Errorf("kube client config: %s", err)
	}

	backup, err := createBackup(tiClient, namespace, cluster)
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
			b, err := tiClient.Mysql().MysqlBackups(namespace).Get(backup.Name, meta.GetOptions{})
			if err != nil {
				glog.Warningf("Failed to get backup: %s", err)
			}
			if i, ok := util.BackupConditionIndex(api.BackupComplete, b.Status.Conditions); ok {
				cond := b.Status.Conditions[i]
				if cond.Status == core.ConditionTrue {
					glog.Infof("Backup '%s' finished.", backup.Name)
					break
				}
			}
		case <-time.After(time.Hour): // TODO: make duration constant
			return fmt.Errorf("timeout occured while waiting for backup: %s", backup.Name)
		}
	}

	return nil
}

func createBackup(tiClient tiClientset.Interface, ns, cluster string) (*api.MysqlBackup, error) {
	randStr := util.RandStringLowerLetters(10)
	return tiClient.Mysql().MysqlBackups(ns).Create(&api.MysqlBackup{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%s-recurent-backup-%s", cluster, randStr),
			Labels: map[string]string{
				"recurrent": "true",
			},
		},
		Spec: api.BackupSpec{
			ClusterName: cluster,
		},
	})
}

func getTitaniumClient() (tiClientset.Interface, error) {
	// Load the users Kubernetes config
	// in cluster use
	kubeCfg, err := kube.KubeConfig("")

	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %s", err.Error())
	}

	// Create a Navigator api client
	tiClient, err := tiClientset.NewForConfig(kubeCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating internal group client: %s", err.Error())
	}

	return tiClient, nil
}
