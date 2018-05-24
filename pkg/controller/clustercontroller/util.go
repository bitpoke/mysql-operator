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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func (c *Controller) instanceForOwnerReference(objectMeta *metav1.ObjectMeta) (*api.MysqlCluster, error) {

	owner := metav1.GetControllerOf(objectMeta)
	if owner == nil {
		return nil, fmt.Errorf("resource does not have a controller.")
	}

	if owner.Kind != api.ResourceKindMysqlCluster || owner.APIVersion != api.SchemeGroupVersion.String() {
		return nil, fmt.Errorf("reference is not mysql cluster resource")
	}

	cluster, err := c.clusterLister.MysqlClusters(objectMeta.Namespace).Get(owner.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting reference for cluster, err: %s", err)
	}

	return cluster, nil
}
