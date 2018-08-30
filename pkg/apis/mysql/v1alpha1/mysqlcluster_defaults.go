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

	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

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

	// ExporterPort is the port on which metrics exporter expose metrics
	ExporterPort = 9104
)

var (

	// ExporterTargetPort is the port on which metrics exporter expose metrics
	ExporterTargetPort intstr.IntOrString
	// MysqlMasterSlaveConfigs contains configs for both master and slave
	MysqlMasterSlaveConfigs map[string]string
)

// SetDefaults sets the defaults for Spec and Status
func (c *MysqlCluster) SetDefaults(opt *options.Options) {

	if len(c.Spec.MysqlVersion) == 0 {
		c.Spec.MysqlVersion = opt.MysqlImageTag
	}

	c.setPodSpecDefaults(&(c.Spec.PodSpec), opt)

	if len(c.Spec.MysqlConf) == 0 {
		c.Spec.MysqlConf = make(MysqlConf)
	}

	if len(c.Spec.MinAvailable) == 0 && c.Spec.Replicas > 1 {
		c.Spec.MinAvailable = defaultMinAvailable
	}

	// configure mysql based on:
	// https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/

	// set innodb-buffer-pool-size if not set
	if _, ok := c.Spec.MysqlConf["innodb-buffer-pool-size"]; !ok {
		if mem := c.Spec.PodSpec.Resources.Requests.Memory(); mem != nil {
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

			c.Spec.MysqlConf["innodb-buffer-pool-size"] = strconv.FormatInt(bufferSize, 10)
		}
	}

	if _, ok := c.Spec.MysqlConf["innodb-log-file-size"]; !ok {
		if mem := c.Spec.PodSpec.Resources.Requests.Memory(); mem != nil {
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

			c.Spec.MysqlConf["innodb-log-file-size"] = strconv.FormatInt(logFileSize, 10)
		}
	}

	c.setVolumeSpecDefaults(&(c.Spec.VolumeSpec))

	ExporterTargetPort = intstr.FromInt(ExporterPort)

	MysqlMasterSlaveConfigs = map[string]string{
		"log-bin":           "/var/lib/mysql/mysql-bin",
		"log-slave-updates": "on",

		"read-only":        "on",
		"skip-slave-start": "on",

		// Crash safe
		"relay-log-info-repository": "TABLE",
		"relay-log-recovery":        "on",

		// https://github.com/github/orchestrator/issues/323#issuecomment-338451838
		"master_info_repository": "TABLE",

		"default-storage-engine":   "InnoDB",
		"gtid-mode":                "on",
		"enforce-gtid-consistency": "on",

		// MyISAM
		"key-buffer-size":        "32M",
		"myisam-recover-options": "FORCE,BACKUP",

		// Safety
		"max-allowed-packet": "16M",
		"max-connect-errors": "1000000",
		"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER," +
			"NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
		"sysdate-is-now": "1",

		// Binary logging
		"expire-logs-days": "14",
		"sync-binlog":      "1",
		"binlog-format":    "ROW",

		// CACHES AND LIMITS
		"tmp-table-size":         "32M",
		"max-heap-table-size":    "32M",
		"query-cache-type":       "0",
		"query-cache-size":       "0",
		"max-connections":        "500",
		"thread-cache-size":      "50",
		"open-files-limit":       "65535",
		"table-definition-cache": "4096",
		"table-open-cache":       "4096",

		// InnoDB
		"innodb-flush-method":            "O_DIRECT",
		"innodb-log-files-in-group":      "2",
		"innodb-flush-log-at-trx-commit": "2",
		"innodb-file-per-table":          "1",

		"character-set-server": "utf8mb4",
		"collation-server":     "utf8mb4_unicode_ci",

		"skip-name-resolve": "on",
		"skip-host-cache":   "on",
	}

}

// SetDefaults for PodSpec
func (c *MysqlCluster) setPodSpecDefaults(spec *PodSpec, opt *options.Options) {
	if len(spec.ImagePullPolicy) == 0 {
		spec.ImagePullPolicy = opt.ImagePullPolicy
	}

	if len(spec.Resources.Requests) == 0 {

		resRequestCPU, errCPU := resource.ParseQuantity(resourceRequestCPU)
		resRequestMemory, errMemory := resource.ParseQuantity(resourceRequestMemory)

		if errMemory == nil && errCPU == nil {
			spec.Resources = apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceCPU:    resRequestCPU,
					apiv1.ResourceMemory: resRequestMemory,
				},
			}
		}
		if errMemory != nil {
			glog.V(2).Infof("cannot parse '%v': %v", resourceRequestMemory, errMemory)
		}
		if errCPU != nil {
			glog.V(2).Infof("cannot parse '%v': %v", resourceRequestCPU, errCPU)
		}
	}

	// set pod antiaffinity to nodes stay away from other nodes.
	if spec.Affinity.PodAntiAffinity == nil {
		spec.Affinity.PodAntiAffinity = &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				core.WeightedPodAffinityTerm{
					Weight: 100,
					PodAffinityTerm: core.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: c.GetLabels(),
						},
					},
				},
			},
		}
	}
}

// SetDefaults for VolumeSpec
func (c *MysqlCluster) setVolumeSpecDefaults(spec *VolumeSpec) {
	if len(spec.AccessModes) == 0 {
		spec.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
	}
	if len(spec.Resources.Requests) == 0 {

		resStorage, err := resource.ParseQuantity(resourceStorage)
		if err == nil {
			spec.Resources = apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceStorage: resStorage,
				},
			}
		} else {
			glog.V(2).Infof("cannot parse '%v': %v", resourceStorage, err)
		}
	}
}
