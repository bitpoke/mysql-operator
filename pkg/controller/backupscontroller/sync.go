package backupscontroller

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	bfactory "github.com/presslabs/titanium/pkg/backupfactory"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, backup *api.MysqlBackup, ns string) error {
	glog.Infof("sync backup: %s", backup.Name)

	if len(backup.ClusterName) == 0 {
		return fmt.Errorf("cluster name is not specified")
	}

	cluster, err := c.clusterLister.MysqlClusters(backup.Namespace).Get(backup.ClusterName)
	if err != nil {
		return fmt.Errorf("cluster not found: %s", err)
	}

	copyBackup := backup.DeepCopy()
	if err := copyBackup.UpdateDefaults(); err != nil {
		return err
	}

	factory := bfactory.New(copyBackup, c.k8client, c.clientset, cluster)
	if err := factory.Sync(ctx); err != nil {
		return fmt.Errorf("sync: %s", err)
	}

	if _, err := c.clientset.Titanium().MysqlBackups(ns).Update(copyBackup); err != nil {
		return fmt.Errorf("backup update: %s", err)
	}

	return nil
}

func (c *Controller) subresourceUpdated(obj interface{}) {
	var objectMeta *metav1.ObjectMeta
	var err error

	switch typedObject := obj.(type) {
	case *batchv1.Job:
		objectMeta = &typedObject.ObjectMeta
	}

	if objectMeta == nil {
		glog.Errorf("Cannot get ObjectMeta for object %#v", obj)
		return
	}

	cluster, err := c.instanceForOwnerReference(objectMeta)
	if err != nil {
		glog.Errorf("cannot get cluster for ObjectMeta, err: %s", err)
		return
	}

	key, err := controllerpkg.KeyFunc(cluster)
	if err != nil {
		glog.Errorf("key func: %s", err)
		return
	}
	c.queue.Add(key)
}
