package clustercontroller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	mccluster "github.com/presslabs/titanium/pkg/mysqlcluster"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *v1alpha1.MysqlCluster, ns string) error {
	c.logger.Infof("sync cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()

	if err := copyCluster.UpdateDefaults(c.opt); err != nil {
		c.logger.Error("failed to set defaults for cluster")
		return err
	}

	fmt.Println(cluster.Spec)
	fmt.Println(copyCluster.Spec)
	if !reflect.DeepEqual(cluster.Spec, copyCluster.Spec) {
		c.logger.Info("now just update defaults")
		_, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster)
		return err
	}

	// mccluster is the mysql cluster factory.
	cl := mccluster.New(copyCluster, c.KubeCli, c.mcclient, ns)
	if err := cl.Sync(ctx); err != nil {
		c.logger.Error("failed to set-up the cluster")
		return err
	}

	if _, err := c.mcclient.Titanium().MysqlClusters(ns).Update(copyCluster); err != nil {
		return err
	}

	return nil
}
