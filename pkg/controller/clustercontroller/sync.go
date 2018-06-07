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
	"reflect"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myfactory "github.com/presslabs/mysql-operator/pkg/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/util/options"
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, cluster *api.MysqlCluster) error {
	glog.Infof("sync cluster: %s", cluster.Name)
	copyCluster := cluster.DeepCopy()
	opt := options.GetOptions()

	if err := copyCluster.UpdateDefaults(opt); err != nil {
		c.recorder.Event(copyCluster, api.EventWarning, api.EventReasonInitDefaultsFailed,
			"faild to set defauls")
		return fmt.Errorf("failed to set defaults for cluster: %s", err)
	}

	if !reflect.DeepEqual(cluster.Spec, copyCluster.Spec) {
		// updating defaults
		glog.V(2).Infof("now just update defaults for %s", cluster.Name)
		copyCluster.UpdateStatusCondition(api.ClusterConditionReady,
			apiv1.ConditionUnknown, "not initialized", "setting defaults")
		copyCluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			apiv1.ConditionUnknown, "not initialized", "setting defaults")

		c.recorder.Event(copyCluster, api.EventNormal, api.EventReasonInitDefaults,
			"defaults set")
		_, err := c.myClient.Mysql().MysqlClusters(cluster.Namespace).Update(copyCluster)
		return err
	}

	// create a cluster factory and sync it.
	clusterFactory := myfactory.New(copyCluster, opt, c.k8client, c.myClient,
		cluster.Namespace, c.recorder)
	if err := clusterFactory.Sync(ctx); err != nil {
		return fmt.Errorf("failed to set-up the cluster: %s", err)
	}

	if _, err := c.myClient.Mysql().MysqlClusters(cluster.Namespace).Update(copyCluster); err != nil {
		return err
	}

	c.registerClusterInReconciliation(cluster)
	c.registerClusterInBackupCron(cluster)

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
		glog.V(4).Infof("Cannot get ObjectMeta for obj: %#v", obj)
		return
	}

	cluster, err := c.instanceForOwnerReference(objectMeta)
	if err != nil {
		glog.V(4).Infof("Cannot get cluster for ObjectMeta, err: %s", err)
		return
	}

	c.addClusterInWorkQueue(cluster)
}
