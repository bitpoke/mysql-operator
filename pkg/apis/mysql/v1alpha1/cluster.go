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
	"strconv"

	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/mysql-operator/pkg/util/options"
)

const (
	innodbBufferSizePercent = 80
)

const (
	_        = iota // ignore first value by assigning to blank identifier
	KB int64 = 1 << (10 * iota)
	MB
	GB
)

var (
	opt *options.Options
)

func init() {
	opt = options.GetOptions()
}

// AsOwnerReference returns the MysqlCluster owner references.
func (c *MysqlCluster) AsOwnerReference() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       ResourceKindMysqlCluster,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}

// UpdateDefaults sets the defaults for Spec and Status
func (c *MysqlCluster) UpdateDefaults(opt *options.Options) error {
	return c.Spec.UpdateDefaults(opt, c)
}

// UpdateDefaults updates Spec defaults
func (c *ClusterSpec) UpdateDefaults(opt *options.Options, cluster *MysqlCluster) error {
	if len(c.MysqlVersion) == 0 {
		c.MysqlVersion = opt.MysqlImageTag
	}

	if err := c.PodSpec.UpdateDefaults(opt, cluster); err != nil {
		return err
	}

	if len(c.MysqlConf) == 0 {
		c.MysqlConf = make(MysqlConf)
	}

	// configure mysql based on:
	// https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/

	// set innodb-buffer-pool-size if not set
	if _, ok := c.MysqlConf["innodb-buffer-pool-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			var bufferSize int64
			if mem.Value() < GB {
				// RAM < 1G => buffer size set to 128M
				bufferSize = 128 * MB
			} else if mem.Value() <= 4*GB {
				// RAM <= 4GB => buffer size set to RAM * 0.5
				bufferSize = int64(float64(mem.Value()) * 0.5)
			} else {
				// RAM > 4GB => buffer size set to RAM * 0.75
				bufferSize = int64(float64(mem.Value()) * 0.75)
			}

			c.MysqlConf["innodb-buffer-pool-size"] = strconv.FormatInt(bufferSize, 10)
		}
	}

	if _, ok := c.MysqlConf["innodb-log-file-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			var logFileSize int64
			if mem.Value() < GB {
				// RAM < 1G
				logFileSize = 48 * MB
			} else if mem.Value() <= 4*GB {
				// RAM <= 4GB
				logFileSize = 128 * MB
			} else if mem.Value() <= 8*GB {
				// RAM <= 8GB
				logFileSize = 512 * GB
			} else if mem.Value() <= 16*GB {
				// RAM <= 16GB
				logFileSize = 1 * GB
			} else {
				// RAM > 16GB
				logFileSize = 2 * GB
			}

			c.MysqlConf["innodb-log-file-size"] = strconv.FormatInt(logFileSize, 10)
		}
	}

	return c.VolumeSpec.UpdateDefaults()
}

// GetHelperImage return helper image from options
func (c *ClusterSpec) GetHelperImage() string {
	return opt.HelperImage
}

// GetMetricsExporterImage return helper image from options
func (c *ClusterSpec) GetMetricsExporterImage() string {
	return opt.MetricsExporterImage
}

// GetOrcUri return the orchestrator uri
func (c *ClusterSpec) GetOrcUri() string {
	return opt.OrchestratorUri
}

// GetMysqlImage returns mysql image, composed from oprions and  Spec.MysqlVersion
func (c *ClusterSpec) GetMysqlImage() string {
	return opt.MysqlImage + ":" + c.MysqlVersion
}

const (
	resourceRequestCPU    = "200m"
	resourceRequestMemory = "1Gi"

	resourceStorage = "1Gi"
)

// UpdateDefaults for PodSpec
func (ps *PodSpec) UpdateDefaults(opt *options.Options, cluster *MysqlCluster) error {
	if len(ps.ImagePullPolicy) == 0 {
		ps.ImagePullPolicy = opt.ImagePullPolicy
	}

	if len(ps.Resources.Requests) == 0 {
		ps.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceCPU:    resource.MustParse(resourceRequestCPU),
				apiv1.ResourceMemory: resource.MustParse(resourceRequestMemory),
			},
		}
	}

	// set pod antiaffinity to nodes stay away from other nodes.
	if ps.Affinity.PodAntiAffinity == nil {
		ps.Affinity.PodAntiAffinity = &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				core.WeightedPodAffinityTerm{
					Weight: 100,
					PodAffinityTerm: core.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: cluster.GetLabels(),
						},
					},
				},
			},
		}
	}
	return nil
}

// UpdateDefaults for VolumeSpec
func (vs *VolumeSpec) UpdateDefaults() error {
	if len(vs.AccessModes) == 0 {
		vs.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
	}

	if len(vs.Resources.Requests) == 0 {
		vs.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceStorage: resource.MustParse(resourceStorage),
			},
		}
	}

	return nil
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
	// HealthyNodes is the name of a service that continas all healthy nodes
	HealthyNodesService ResourceName = "healthy-nodes-service"
)

func (c *MysqlCluster) GetNameForResource(name ResourceName) string {
	return GetNameForResource(name, c.Name)
}

func GetNameForResource(name ResourceName, clusterName string) string {
	switch name {
	case StatefulSet, ConfigMap, BackupCronJob, HealthyNodesService:
		return fmt.Sprintf("%s-mysql", clusterName)
	case MasterService:
		return fmt.Sprintf("%s-mysql-master", clusterName)
	case HeadlessSVC:
		return fmt.Sprintf("%s-mysql-nodes", clusterName)
	default:
		return fmt.Sprintf("%s-mysql", clusterName)
	}
}

// GetBackupCandidate returns the hostname of the first not-lagged and
// replicating slave node, else returns the master node.
func (c *MysqlCluster) GetBackupCandidate() string {
	for _, node := range c.Status.Nodes {
		master := node.GetCondition(NodeConditionMaster)
		replicating := node.GetCondition(NodeConditionReplicating)
		lagged := node.GetCondition(NodeConditionLagged)
		if master == nil || replicating == nil || lagged == nil {
			continue
		}
		if master.Status == core.ConditionFalse &&
			replicating.Status == core.ConditionTrue &&
			lagged.Status == core.ConditionFalse {
			return node.Name
		}
	}
	glog.Warningf("No healthy slave node found so returns the master node: %s.", c.GetPodHostname(0))
	return c.GetPodHostname(0)
}

func (c *MysqlCluster) GetPodHostname(p int) string {
	return fmt.Sprintf("%s-%d.%s.%s", c.GetNameForResource(StatefulSet), p,
		c.GetNameForResource(HeadlessSVC),
		c.Namespace)
}

func (c *MysqlCluster) GetLabels() map[string]string {
	return map[string]string{
		"app":           "mysql-operator",
		"mysql_cluster": c.Name,
	}
}
