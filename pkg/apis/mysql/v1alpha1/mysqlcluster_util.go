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

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// BackupCronJob is the name of cron job
	BackupCronJob ResourceName = "backup-cron"
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
	case StatefulSet, ConfigMap, BackupCronJob, HealthyNodesService, PodDisruptionBudget:
		return fmt.Sprintf("%s-mysql", clusterName)
	case MasterService:
		return fmt.Sprintf("%s-mysql-master", clusterName)
	case HeadlessSVC:
		return fmt.Sprintf("%s-mysql-nodes", clusterName)
	default:
		return fmt.Sprintf("%s-mysql", clusterName)
	}
}

// GetPodHostname returns for an index the pod hostname of a cluster
func (c *MysqlCluster) GetPodHostname(p int) string {
	return fmt.Sprintf("%s-%d.%s.%s", c.GetNameForResource(StatefulSet), p,
		c.GetNameForResource(HeadlessSVC),
		c.Namespace)
}

// GetClusterAlias returns the cluster alias that as it is in orchestrator
func (c *MysqlCluster) GetClusterAlias() string {
	return fmt.Sprintf("%s.%s", c.Name, c.Namespace)
}

// GetMasterHost returns name of current master host in a cluster
func (c *MysqlCluster) GetMasterHost() string {
	masterHost := c.GetPodHostname(0)

	for _, ns := range c.Status.Nodes {
		if cond := ns.GetCondition(NodeConditionMaster); cond != nil &&
			cond.Status == core.ConditionTrue {
			masterHost = ns.Name
		}
	}

	return masterHost
}

// AsOwnerReference returns the MysqlCluster owner references.
func (c *MysqlCluster) AsOwnerReference() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "MysqlCluster",
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}
