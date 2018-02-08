package clustercontroller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"

	"github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	mccluster "github.com/presslabs/titanium/pkg/mysqlcluster"
	"github.com/presslabs/titanium/pkg/util/options"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *v1alpha1.MysqlCluster, ns string) error {
	glog.Infof("sync cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	if err := copyCluster.UpdateDefaults(opt); err != nil {
		copyCluster.UpdateStatusCondition(v1alpha1.ClusterConditionInitDefaults,
			apiv1.ConditionFalse, "not initialized", err.Error())
		if _, err2 := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err2 != nil {
			return fmt.Errorf("update failes, errors:: %s + %s", err2, err)
		}
		return fmt.Errorf("failed to set defaults for cluster: %s", err)
	}

	if !reflect.DeepEqual(cluster.Spec, copyCluster.Spec) {
		// updating defaults
		glog.V(2).Info("now just update defaults for %s", cluster.Name)
		copyCluster.UpdateStatusCondition(v1alpha1.ClusterConditionInitDefaults,
			apiv1.ConditionTrue, "not initialized", "set defaults")
		copyCluster.UpdateStatusCondition(v1alpha1.ClusterConditionReady,
			apiv1.ConditionUnknown, "not initialized", "set defaults")
		_, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster)
		return err
	}

	// mccluster is the mysql cluster factory.
	cl := mccluster.New(copyCluster, c.KubeCli, c.mcclient, ns)
	if err := cl.Sync(ctx); err != nil {
		return fmt.Errorf("failed to set-up the cluster: %s", err)
	}

	if _, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err != nil {
		return err
	}

	return nil
}
