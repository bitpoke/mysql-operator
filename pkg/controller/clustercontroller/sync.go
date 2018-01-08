package clustercontroller

import (
	"context"

	"github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
)

func (c *Controller) Sync(ctx context.Context, cluster *v1alpha1.MysqlCluster) error {
	c.logger.Info("Called sync cluster! ", cluster)
	return nil
}
