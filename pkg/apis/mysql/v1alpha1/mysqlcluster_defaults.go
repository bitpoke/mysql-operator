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
	apiv1 "k8s.io/api/core/v1"
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

// SetDefaults_MysqlCluster sets the defaults for a mysqlcluster object
// nolint
func SetDefaults_MysqlCluster(c *MysqlCluster) {

	c.setPodSpecDefaults(&(c.Spec.PodSpec))
	c.setVolumeSpecDefaults(&(c.Spec.VolumeSpec))

	if c.Spec.Replicas == nil {
		one := int32(1)
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
		spec.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceCPU:    resourceRequestCPU,
				apiv1.ResourceMemory: resourceRequestMemory,
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
		spec.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceStorage: resourceStorage,
			},
		}
	}
}
