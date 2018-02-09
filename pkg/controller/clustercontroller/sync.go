package clustercontroller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	mccluster "github.com/presslabs/titanium/pkg/mysqlcluster"
	"github.com/presslabs/titanium/pkg/util/options"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *api.MysqlCluster, ns string) error {
	glog.Infof("sync cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	if err := copyCluster.UpdateDefaults(opt); err != nil {
		//		copyCluster.UpdateStatusCondition(api.ClusterConditionInitDefaults,
		//			apiv1.ConditionFalse, "not initialized", err.Error())
		//		if _, err2 := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err2 != nil {
		//			return fmt.Errorf("update failes, errors:: %s + %s", err2, err)
		//		}
		c.recorder.Event(copyCluster, api.EventWarning, api.EventReasonInitDefaultsFaild,
			"faild to set defauls")
		return fmt.Errorf("failed to set defaults for cluster: %s", err)
	}

	if !reflect.DeepEqual(cluster.Spec, copyCluster.Spec) {
		// updating defaults
		glog.V(2).Infof("now just update defaults for %s", cluster.Name)
		//copyCluster.UpdateStatusCondition(api.ClusterConditionInitDefaults,
		//	apiv1.ConditionTrue, "not initialized", "set defaults")
		copyCluster.UpdateStatusCondition(api.ClusterConditionReady,
			apiv1.ConditionUnknown, "not initialized", "setting defaults")
		c.recorder.Event(copyCluster, api.EventNormal, api.EventReasonInitDefaults,
			"defaults seted")
		_, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster)
		return err
	}

	// mccluster is the mysql cluster factory.
	cl := mccluster.New(copyCluster, c.KubeCli, c.mcclient, ns, c.recorder)
	if err := cl.Sync(ctx); err != nil {
		return fmt.Errorf("failed to set-up the cluster: %s", err)
	}

	if _, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err != nil {
		return err
	}

	return nil
}
