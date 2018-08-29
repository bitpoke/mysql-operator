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

package mysqlcluster

import (
	"fmt"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// MysqlCluster is the wrapper for api.MysqlCluster type
type MysqlCluster struct {
	*api.MysqlCluster
}

// NewMysqlClusterWrapper returns a wrapper for mysqlcluster
func NewMysqlClusterWrapper(mc *api.MysqlCluster) *MysqlCluster {
	return &MysqlCluster{
		MysqlCluster: mc,
	}
}

// GetPodHostname returns for an index the pod hostname of a cluster
func (c *MysqlCluster) GetPodHostname(p int) string {
	return fmt.Sprintf("%s-%d.%s.%s", c.MysqlCluster.GetNameForResource(api.StatefulSet), p,
		c.MysqlCluster.GetNameForResource(api.HeadlessSVC),
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
		if cond := c.GetNodeCondition(ns.Name, api.NodeConditionMaster); cond != nil &&
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
		APIVersion: api.SchemeGroupVersion.String(),
		Kind:       "MysqlCluster",
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}
