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

package v1alpha1

import (
	"fmt"
)

// GetLabels returns cluster labels
func (c *MysqlCluster) GetLabels() map[string]string {
	return map[string]string{
		"app":           "mysql-operator",
		"mysql_cluster": c.Name,
	}
}

// ResourceName is the type for aliasing resources that will be created.
type ResourceName string

const (
	// HeadlessSVC is the alias of the headless service resource
	HeadlessSVC ResourceName = "headless"
	// StatefulSet is the alias of the statefulset resource
	StatefulSet ResourceName = "mysql"
	// ConfigMap is the alias for mysql configs, the config map resource
	ConfigMap ResourceName = "config-files"
	// MasterService is the name of the service that points to master node
	MasterService ResourceName = "master-service"
	// HealthyNodesService is the name of a service that continas all healthy nodes
	HealthyNodesService ResourceName = "healthy-nodes-service"
	// PodDisruptionBudget is the name of pod disruption budget for the stateful set
	PodDisruptionBudget ResourceName = "pdb"
)

// GetNameForResource returns the name of a resource from above
func (c *MysqlCluster) GetNameForResource(name ResourceName) string {
	return GetNameForResource(name, c.Name)
}

// GetNameForResource returns the name of a resource for a cluster
func GetNameForResource(name ResourceName, clusterName string) string {
	switch name {
	case StatefulSet, ConfigMap, HealthyNodesService, PodDisruptionBudget:
		return fmt.Sprintf("%s-mysql", clusterName)
	case MasterService:
		return fmt.Sprintf("%s-mysql-master", clusterName)
	case HeadlessSVC:
		return fmt.Sprintf("%s-mysql-nodes", clusterName)
	default:
		return fmt.Sprintf("%s-mysql", clusterName)
	}
}
