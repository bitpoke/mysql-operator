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
	"github.com/presslabs/mysql-operator/pkg/options"
	"strings"

	"github.com/blang/semver"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

const (
	// HeadlessSVCName is the name of the headless service that is commonly used for all clusters
	HeadlessSVCName = "mysql"
)

// MysqlCluster is the wrapper for api.MysqlCluster type
type MysqlCluster struct {
	*api.MysqlCluster
}

// NodeInitializedConditionType is the extended new pod condition that marks the pod as initialized from MySQL
// point of view.
const NodeInitializedConditionType core.PodConditionType = "mysql.presslabs.org/NodeInitialized"

// New returns a wrapper for mysqlcluster
func New(mc *api.MysqlCluster) *MysqlCluster {
	return &MysqlCluster{
		MysqlCluster: mc,
	}
}

// Unwrap returns the api mysqlcluster object
func (c *MysqlCluster) Unwrap() *api.MysqlCluster {
	return c.MysqlCluster
}

// GetLabels returns cluster labels
func (c *MysqlCluster) GetLabels() labels.Set {

	instance := c.Name
	if inst, ok := c.Annotations["app.kubernetes.io/instance"]; ok {
		instance = inst
	}

	component := "database"
	if comp, ok := c.Annotations["app.kubernetes.io/component"]; ok {
		component = comp
	}

	labels := labels.Set{
		"mysql.presslabs.org/cluster": c.Name,

		"app.kubernetes.io/name":       "mysql",
		"app.kubernetes.io/instance":   instance,
		"app.kubernetes.io/version":    c.GetMySQLSemVer().String(),
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "mysql.presslabs.org",
	}

	if part, ok := c.Annotations["app.kubernetes.io/part-of"]; ok {
		labels["app.kubernetes.io/part-of"] = part
	}

	return labels
}

// GetSelectorLabels returns the labels that will be used as selector
func (c *MysqlCluster) GetSelectorLabels() labels.Set {
	return labels.Set{
		"mysql.presslabs.org/cluster": c.Name,

		"app.kubernetes.io/name":       "mysql",
		"app.kubernetes.io/managed-by": "mysql.presslabs.org",
	}
}

// ResourceName is the type for aliasing resources that will be created.
type ResourceName string

const (
	// OldHeadlessSVC is the name of the old headless service
	// DEPRECATED
	OldHeadlessSVC = "old-headless"

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
	// Secret is the name of the "private" secret that contains operator related credentials
	Secret ResourceName = "operated-secret"
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
		return HeadlessSVCName
	case OldHeadlessSVC:
		return fmt.Sprintf("%s-mysql-nodes", clusterName)
	case Secret:
		return fmt.Sprintf("%s-mysql-operated", clusterName)
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
		if cond := c.GetNodeCondition(ns.Name, api.NodeConditionMaster); cond != nil &&
			cond.Status == core.ConditionTrue {
			masterHost = ns.Name
		}
	}

	return masterHost
}

// GetMySQLSemVer returns the MySQL server version in semver format, or the default one
func (c *MysqlCluster) GetMySQLSemVer() semver.Version {
	version := c.Spec.MysqlVersion
	// lookup for an alias, usually this will solve 5.7 to 5.7.x
	if v, ok := constants.MySQLTagsToSemVer[version]; ok {
		version = v
	}

	sv, err := semver.Make(version)
	if err != nil {
		log.Error(err, "failed to parse given MySQL version", "input", version)
	}

	// if there is an error will return 0.0.0
	return sv
}

// GetMysqlImage returns the mysql image for current mysql cluster
func (c *MysqlCluster) GetMysqlImage() string {
	if len(c.Spec.Image) != 0 {
		return c.Spec.Image
	}

	// check if the user set some overrides
	opt := options.GetOptions()
	if img, ok := opt.MySQLVersionImageOverride[c.GetMySQLSemVer().String()]; ok {
		return img
	}

	if img, ok := constants.MysqlImageVersions[c.GetMySQLSemVer().String()]; ok {
		return img
	}

	// this means the cluster has a wrong MysqlVersion set
	log.Error(nil, "no image found with given MySQL version, the image can manually be set by setting .spec.mysqlImage on cluster",
		"version", c.GetMySQLSemVer())
	return ""
}

// UpdateSpec updates the cluster specs that need to be saved
func (c *MysqlCluster) UpdateSpec() {
	// TODO: remove this in next major release (v0.4)
	if len(c.Spec.InitBucketURL) == 0 {
		c.Spec.InitBucketURL = c.Spec.InitBucketURI
	}
}

// ShouldHaveInitContainerForMysql checks the MySQL version and returns true or false if the docker image supports or not init only
func (c *MysqlCluster) ShouldHaveInitContainerForMysql() bool {
	expectedRange := semver.MustParseRange(">=5.7.26 <8.0.0 || >=8.0.15")

	return strings.Contains(c.GetMysqlImage(), "percona") && expectedRange(c.GetMySQLSemVer())
}
