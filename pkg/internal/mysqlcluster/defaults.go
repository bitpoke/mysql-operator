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
	"math"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

// nolint: megacheck, deadcode, varcheck
const (
	_        = iota // ignore first value by assigning to blank identifier
	kb int64 = 1 << (10 * iota)
	mb
	gb
)

// SetDefaults set defaults from options
// nolint: gocyclo
func (cluster *MysqlCluster) SetDefaults(opt *options.Options) {
	// set default image pull policy
	if len(cluster.Spec.PodSpec.ImagePullPolicy) == 0 {
		cluster.Spec.PodSpec.ImagePullPolicy = opt.ImagePullPolicy
	}

	// set default image pull secrets
	if len(cluster.Spec.PodSpec.ImagePullSecrets) == 0 {
		if len(opt.ImagePullSecretName) != 0 {
			cluster.Spec.PodSpec.ImagePullSecrets = []core.LocalObjectReference{
				{Name: opt.ImagePullSecretName},
			}
		}
	}

	// set mysql version if not set to avoid spamming logs
	if len(cluster.Spec.MysqlVersion) == 0 {
		cluster.Spec.MysqlVersion = constants.MySQLDefaultVersion.String()
	}

	// set pod antiaffinity to nodes stay away from other nodes.
	if cluster.Spec.PodSpec.Affinity == nil {
		cluster.Spec.PodSpec.Affinity = &core.Affinity{
			PodAntiAffinity: &core.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: core.PodAffinityTerm{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: cluster.GetLabels(),
							},
						},
					},
				},
			},
		}
	}

	// configure mysql based on:
	// https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/

	// set innodb-buffer-pool-size if not set
	innodbBufferPoolSize := 128 * mb // MySQL default value
	if mem := cluster.Spec.PodSpec.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
		var cErr error
		if innodbBufferPoolSize, cErr = computeInnodbBufferPoolSize(mem); cErr == nil {
			setConfigIfNotSet(cluster.Spec.MysqlConf, "innodb-buffer-pool-size", humanizeSize(innodbBufferPoolSize))
		}
	}

	if mem := cluster.Spec.PodSpec.Resources.Requests.Memory(); mem != nil {
		logFileSize := humanizeSize(computeInnodbLogFileSize(mem))
		setConfigIfNotSet(cluster.Spec.MysqlConf, "innodb-log-file-size", logFileSize)
	}

	if pvc := cluster.Spec.VolumeSpec.PersistentVolumeClaim; pvc != nil {
		if space := getRequestedStorage(pvc); space != nil {
			binlogSpaceLimit := space.Value() / 2
			maxBinlogSize := min(binlogSpaceLimit/4, 1*gb)
			if space.Value() < 2*gb {
				binlogSpaceLimit = space.Value() / 3
				maxBinlogSize = min(binlogSpaceLimit/3, 1*gb)
			}

			// binlog-space-limit = totalSpace / 2
			setConfigIfNotSet(cluster.Spec.MysqlConf, "binlog-space-limit", humanizeSize(binlogSpaceLimit))

			// max-binlog-size = min(binlog-space-limit / 4, 1*gb)
			setConfigIfNotSet(cluster.Spec.MysqlConf, "max-binlog-size", humanizeSize(maxBinlogSize))
		}
	}

	if cpu := cluster.Spec.PodSpec.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
		// innodb_buffer_pool_instances = min(ceil(resources.limits.cpu), floor(innodb_buffer_pool_size/1Gi))
		cpuRounded := math.Ceil(float64(cpu.MilliValue()) / float64(1000))
		instances := math.Max(math.Min(cpuRounded, math.Floor(float64(innodbBufferPoolSize)/float64(gb))), 1)
		setConfigIfNotSet(cluster.Spec.MysqlConf, "innodb-buffer-pool-instances", intstr.FromInt(int(instances)))
	}

	// set default xtrabackup target directory
	if len(cluster.Spec.XtrabackupTargetDir) == 0 {
		cluster.Spec.XtrabackupTargetDir = "/tmp/xtrabackup_backupfiles/"
	}
}

func setConfigIfNotSet(conf api.MysqlConf, option string, value intstr.IntOrString) {
	if _, ok := conf[option]; !ok {
		conf[option] = value
	}
}

func getRequestedStorage(pvc *core.PersistentVolumeClaimSpec) *resource.Quantity {
	if val, ok := pvc.Resources.Requests[core.ResourceStorage]; ok {
		return &val
	}
	return nil
}

func humanizeSize(value int64) intstr.IntOrString {

	if r, ok := tryUnit(value, gb, "G"); ok {
		return intstr.FromString(r)
	}

	if r, ok := tryUnit(value, mb, "M"); ok {
		return intstr.FromString(r)
	}

	if r, ok := tryUnit(value, kb, "K"); ok {
		return intstr.FromString(r)
	}

	return intstr.FromString(fmt.Sprintf("%d%s", value, ""))
}

func tryUnit(value int64, scale int64, unitStr string) (string, bool) {
	allow := value * 1 / 1000
	if value >= scale && value%scale <= allow {
		value /= scale
		return fmt.Sprintf("%d%s", value, unitStr), true
	}

	return "", false
}

// computeInnodbLogFileSize returns a computed value, to configure MySQL, based on requested memory.
func computeInnodbLogFileSize(mem *resource.Quantity) int64 {
	var logFileSize int64
	if mem.Value() < gb {
		// RAM < 1G
		logFileSize = 48 * mb
	} else if mem.Value() <= 4*gb {
		// RAM <= 4gb
		logFileSize = 128 * mb
	} else if mem.Value() <= 8*gb {
		// RAM <= 8gb
		logFileSize = 512 * mb
	} else if mem.Value() <= 16*gb {
		// RAM <= 16gb
		logFileSize = 1 * gb
	} else {
		// RAM > 16gb
		logFileSize = 2 * gb
	}

	return logFileSize
}

// computeInnodbBufferPoolSize returns a computed value, to configure MySQL, based on requested
// memory. As described in: https://github.com/presslabs/mysql-operator/issues/502
func computeInnodbBufferPoolSize(mem *resource.Quantity) (int64, error) {
	availableMem := mem.Copy()
	percentRAM := 0.75

	if mem.Value() <= 512*mb {
		// don't set innodb-buffer-pool-size leave it to mysql default (128M)
		return 0, fmt.Errorf("memory too small to compute innodb-buffer-pool-size: %s", mem)
	} else if mem.Value() <= 1*gb {
		// RAM <= 1gb => buffer size set to RAM * 0.5
		availableMem.Sub(resource.MustParse("256Mi"))
		percentRAM = 0.5
	} else if mem.Value() <= 4*gb {
		// RAM <= 4gb => buffer size set to RAM * 0.75
		availableMem.Sub(resource.MustParse("256Mi"))
		percentRAM = 0.75
	} else if mem.Value() > 4*gb {
		// RAM > 4gb => buffer size set to RAM * 0.8
		availableMem.Sub(resource.MustParse("512Mi"))
		percentRAM = 0.8
	}

	return int64(float64(availableMem.Value()) * percentRAM), nil
}

func min(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}
