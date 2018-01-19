package clustercontroller

import (
	"context"

	"github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	mccluster "github.com/presslabs/titanium/pkg/mysqlcluster"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *v1alpha1.MysqlCluster, ns string) error {
	c.logger.Info("Called sync cluster!")

	if err := cluster.UpdateDefaults(); err != nil {
		c.logger.Error("Failed to set defaults for cluster.")
		return err
	}
	cl := mccluster.New(cluster, c.KubeCli, c.mcclient, ns)

	err := cl.Sync(ctx)
	if err != nil {
		c.logger.Error("Failed to set-up the cluster.!")
		return err
	}

	// update cluster defaults and state.
	if _, err := c.mcclient.Titanium().MysqlClusters(ns).Update(cluster); err != nil {
		return err
	}

	return nil
}
