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

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myfactory "github.com/presslabs/mysql-operator/pkg/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/util/options"
)

const (
	reconcileTime = 1 * time.Second
)

func (c *Controller) Reconcile(ctx context.Context, cluster *api.MysqlCluster) error {
	// ensure that cluster will be reconciled
	defer c.addClusterInReconcileQueue(cluster, reconcileTime)

	glog.V(1).Infof("reconcile cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	clusterFactory := myfactory.New(copyCluster, opt, c.k8client, c.myClient, cluster.Namespace, c.recorder, c.podLister)
	if err := clusterFactory.Reconcile(ctx); err != nil {
		return fmt.Errorf("failed to reconcile the cluster: %s", err)
	}

	return nil
}
