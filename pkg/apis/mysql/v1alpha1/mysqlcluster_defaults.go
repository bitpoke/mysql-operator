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
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/mysql-operator/pkg/options"
)

// nolint: deadcode, varcheck
const (
	_        = iota // ignore first value by assigning to blank identifier
	kb int64 = 1 << (10 * iota)
	mb
	gb
)

const (
	resourceRequestCPU    = "200m"
	resourceRequestMemory = "1Gi"

	resourceStorage = "1Gi"

	defaultMinAvailable = "50%"
)

// SetDefaults sets the defaults for Spec and Status
func (c *MysqlCluster) SetDefaults(opt *options.Options) error {
	return c.Spec.SetDefaults(opt, c)
}

// SetDefaults updates Spec defaults
// nolint: gocyclo
func (c *MysqlClusterSpec) SetDefaults(opt *options.Options, cluster *MysqlCluster) error {
	if len(c.MysqlVersion) == 0 {
		c.MysqlVersion = opt.MysqlImageTag
	}

	if err := c.PodSpec.SetDefaults(opt, cluster); err != nil {
		return err
	}

	if len(c.MysqlConf) == 0 {
		c.MysqlConf = make(MysqlConf)
	}

	if len(c.MinAvailable) == 0 && c.Replicas > 1 {
		c.MinAvailable = defaultMinAvailable
	}

	// configure mysql based on:
	// https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/

	// set innodb-buffer-pool-size if not set
	if _, ok := c.MysqlConf["innodb-buffer-pool-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			var bufferSize int64
			if mem.Value() < gb {
				// RAM < 1G => buffer size set to 128M
				bufferSize = 128 * mb
			} else if mem.Value() <= 4*gb {
				// RAM <= 4gb => buffer size set to RAM * 0.5
				bufferSize = int64(float64(mem.Value()) * 0.5)
			} else {
				// RAM > 4gb => buffer size set to RAM * 0.75
				bufferSize = int64(float64(mem.Value()) * 0.75)
			}

			c.MysqlConf["innodb-buffer-pool-size"] = strconv.FormatInt(bufferSize, 10)
		}
	}

	if _, ok := c.MysqlConf["innodb-log-file-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			var logFileSize int64
			if mem.Value() < gb {
				// RAM < 1G
				logFileSize = 48 * mb
			} else if mem.Value() <= 4*gb {
				// RAM <= 4gb
				logFileSize = 128 * mb
			} else if mem.Value() <= 8*gb {
				// RAM <= 8gb
				logFileSize = 512 * gb
			} else if mem.Value() <= 16*gb {
				// RAM <= 16gb
				logFileSize = 1 * gb
			} else {
				// RAM > 16gb
				logFileSize = 2 * gb
			}

			c.MysqlConf["innodb-log-file-size"] = strconv.FormatInt(logFileSize, 10)
		}
	}

	return c.VolumeSpec.SetDefaults()
}

// SetDefaults for PodSpec
func (ps *PodSpec) SetDefaults(opt *options.Options, cluster *MysqlCluster) error {
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

// SetDefaults for VolumeSpec
func (vs *VolumeSpec) SetDefaults() error {
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
