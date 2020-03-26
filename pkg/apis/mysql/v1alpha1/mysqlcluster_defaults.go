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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultMinAvailable = "50%"
)

var (
	resourceStorage       = resource.MustParse("1Gi")
	resourceRequestCPU    = resource.MustParse("200m")
	resourceRequestMemory = resource.MustParse("1Gi")
)

var (
	one int32 = 1
)

// SetDefaults_MysqlCluster sets the defaults for a MySQLCLuster object
// nolint
func SetDefaults_MysqlCluster(c *MysqlCluster) {

	c.setPodSpecDefaults(&(c.Spec.PodSpec))

	if c.Spec.VolumeSpec.PersistentVolumeClaim != nil {
		c.setVolumeSpecDefaults(c.Spec.VolumeSpec.PersistentVolumeClaim)
	}

	if c.Spec.Replicas == nil {
		c.Spec.Replicas = &one
	}

	if len(c.Spec.MysqlConf) == 0 {
		c.Spec.MysqlConf = make(MysqlConf)
	}

	if len(c.Spec.MinAvailable) == 0 && *c.Spec.Replicas > 1 {
		c.Spec.MinAvailable = defaultMinAvailable
	}
}

// SetDefaults for PodSpec
func (c *MysqlCluster) setPodSpecDefaults(spec *PodSpec) {
	if len(spec.Resources.Requests) == 0 {
		spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resourceRequestCPU,
				corev1.ResourceMemory: resourceRequestMemory,
			},
		}
	}

	if len(spec.MetricsExporterResources.Requests) == 0 {
		spec.MetricsExporterResources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		}
	}

	if len(spec.MetricsExporterResources.Limits) == 0 {
		spec.MetricsExporterResources.Limits = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		}
	}

	if len(spec.MySQLOperatorSidecarResources.Requests) == 0 {
		spec.MySQLOperatorSidecarResources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}
	}

	if len(spec.MySQLOperatorSidecarResources.Limits) == 0 {
		// for the sidecar set the same limits as for mysql
		spec.MySQLOperatorSidecarResources.Limits = spec.Resources.Limits
	}
}

// SetDefaults for VolumeSpec
func (c *MysqlCluster) setVolumeSpecDefaults(spec *corev1.PersistentVolumeClaimSpec) {
	if len(spec.AccessModes) == 0 {
		spec.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		}
	}
	if len(spec.Resources.Requests) == 0 {
		spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resourceStorage,
			},
		}
	}
}
