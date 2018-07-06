/*
Copyright 2018 Platform9, Inc.

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
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/golang/glog"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	clusterpkg "github.com/presslabs/mysql-operator/pkg/mysqlcluster"
)

func (c *Controller) registerClusterCleanup(cluster *api.MysqlCluster) {
	key, err := controllerpkg.KeyFunc(cluster)

	if err != nil {
		runtime.HandleError(err)
		return
	}

	_, loaded := c.clustersCleanup.LoadOrStore(key, *cluster)

	if !loaded {
		glog.V(2).Infof("Registered cluster '%s' for cleanup.", key)
	}
}

func (c *Controller) cleanupCluster(key string) {
	val, exists := c.clustersCleanup.Load(key)
	if !exists {
		glog.V(2).Infof("Cluster '%s' parameters not found, skipping cleanup", key)
		return
	}

	if cluster, ok := val.(api.MysqlCluster); ok {
		clusterpkg.CleanupVolumeClaims(&c.k8client, &cluster)
		c.clustersCleanup.Delete(key)
	} else {
		glog.Errorf("Cluster '%s' cleanup parameters invalid, skipping cleanup", key)
	}
}
