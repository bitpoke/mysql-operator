package appschedulebackup

import (
	"fmt"

	"github.com/golang/glog"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	tiClientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	"github.com/presslabs/titanium/pkg/util"
	"github.com/presslabs/titanium/pkg/util/kube"
)

func RunCommand(stopCh <-chan struct{}, namespace, cluster string) error {
	glog.Infof("Schedule backup for cluster: %s", cluster)
	tiClient, err := getTitaniumClient()
	if err != nil {
		return fmt.Errorf("kube client config: %s", err)
	}

	_, err = createBackup(tiClient, namespace, cluster)
	if err != nil {
		return fmt.Errorf("create backup: %s", err)
	}

	// TODO: wait for backup to finish. Waiting for it guarantees that two
	// backups will not overlap.

	return nil
}

func createBackup(tiClient tiClientset.Interface, ns, cluster string) (*api.MysqlBackup, error) {
	randStr := util.RandStringLowerLetters(10)
	return tiClient.Titanium().MysqlBackups(ns).Create(&api.MysqlBackup{
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
