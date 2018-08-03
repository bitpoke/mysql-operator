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

package clustercontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	myfactory "github.com/presslabs/mysql-operator/pkg/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/util/options"
)

var (
	reconcileTime = 5 * time.Second
)

func (c *Controller) Reconcile(ctx context.Context, cluster *api.MysqlCluster) error {
	// ensure that cluster will be reconciled
	defer c.addClusterInReconcileQueue(cluster, reconcileTime)

	glog.V(1).Infof("Reconciling cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	clusterFactory := myfactory.New(copyCluster, opt, c.k8client, c.myClient, cluster.Namespace, c.recorder)
	if err := clusterFactory.SyncOrchestratorStatus(ctx); err != nil {
		return fmt.Errorf("failed to sync orchestartoe status for cluster '%s': %s", cluster.Name, err)
	}

	if err := clusterFactory.Reconcile(ctx); err != nil {
		return fmt.Errorf("failed to reconcile the cluster '%s': %s", cluster.Name, err)
	}

	if _, err := c.myClient.Mysql().MysqlClusters(cluster.Namespace).Update(copyCluster); err != nil {
		return err
	}

	return nil
}

func (c *Controller) addClusterInReconcileQueue(cluster *api.MysqlCluster, after time.Duration) {
	key, err := controllerpkg.KeyFunc(cluster)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.reconcileQueue.AddAfter(key, after)
}

func (c *Controller) registerClusterInReconciliation(cluster *api.MysqlCluster) {
	key, err := controllerpkg.KeyFunc(cluster)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	_, loaded := c.clustersSync.LoadOrStore(key, true)

	if !loaded {
		glog.V(2).Infof("Register cluster '%s' in reconcile queue.", key)
		// add once a cluster in reconcile loop
		c.addClusterInReconcileQueue(cluster, reconcileTime)
	}
}

func (c *Controller) removeClusterFromReconciliation(key string) {
	c.clustersSync.Delete(key)
}
