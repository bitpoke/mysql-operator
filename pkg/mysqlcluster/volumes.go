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

package mysqlcluster

import (
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// CleanupVolumeClaims deletes persistent volume claims of mysql cluster if needed
func CleanupVolumeClaims(client *kubernetes.Interface, cluster *api.MysqlCluster) error {
	cleanupGracePeriod := cluster.Spec.VolumeSpec.CleanupGracePeriod
	if cleanupGracePeriod <= 0 {
		return nil
	}

	timer := time.NewTimer(time.Duration(cleanupGracePeriod) * time.Second)

	go func(client *kubernetes.Interface, cluster *api.MysqlCluster) {
		<-timer.C
		name := cluster.Name
		glog.V(2).Info("Cleaning up volume claims for cluster: %s", name)

		// Get all volume claims in the cluster
		claimList, err := getVolumeClaimsForCluster(client, cluster.Namespace, cluster.GetLabels(), name)

		if err != nil {
			glog.Errorf("Error deleting volume claims for cluster %s: %s", name, err)
			return
		}

		api := (*client).CoreV1()
		for _, claim := range claimList.Items {
			api.PersistentVolumeClaims(cluster.Namespace).Delete(claim.Name, &metav1.DeleteOptions{})
		}

		glog.V(2).Info("Cleaned up all volume claims for cluster: %s", name)

	}(client, cluster)

	return nil
}
