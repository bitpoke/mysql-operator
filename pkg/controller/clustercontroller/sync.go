package clustercontroller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	mcfactory "github.com/presslabs/titanium/pkg/mysqlcluster"
	"github.com/presslabs/titanium/pkg/util/options"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *api.MysqlCluster, ns string) error {
	glog.Infof("sync cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	if err := copyCluster.UpdateDefaults(opt); err != nil {
		c.recorder.Event(copyCluster, api.EventWarning, api.EventReasonInitDefaultsFaild,
			"faild to set defauls")
		return fmt.Errorf("failed to set defaults for cluster: %s", err)
	}

	if !reflect.DeepEqual(cluster.Spec, copyCluster.Spec) {
		// updating defaults
		copyCluster.UpdateStatusCondition(api.ClusterConditionReady,
			apiv1.ConditionUnknown, "not initialized", "setting defaults")

		glog.V(2).Infof("now just update defaults for %s", cluster.Name)
		c.recorder.Event(copyCluster, api.EventNormal, api.EventReasonInitDefaults,
			"defaults seted")
		_, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster)
		return err
	}

	// create a cluster factory and sync it.
	clusterFactory := mcfactory.New(copyCluster, c.KubeCli, c.mcclient, ns, c.recorder)
	if err := clusterFactory.Sync(ctx); err != nil {
		return fmt.Errorf("failed to set-up the cluster: %s", err)
	}

	if _, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err != nil {
		return err
	}

	return nil
}

func (c *Controller) subresourceUpdated(obj interface{}) {
	var objectMeta *metav1.ObjectMeta
	var err error

	switch typedObject := obj.(type) {
	case *appsv1.StatefulSet:
		objectMeta = &typedObject.ObjectMeta
	}

	if objectMeta == nil {
		runtime.HandleError(fmt.Errorf("Cannot get ObjectMeta for object %#v", obj))
		return
	}

	cluster, err := c.instanceForOwnerReference(objectMeta)
	if err != nil {
		runtime.HandleError(fmt.Errorf("cannot get cluster for ObjectMeta, err: %s", err))
		return
	}

	key, err := controllerpkg.KeyFunc(cluster)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}
