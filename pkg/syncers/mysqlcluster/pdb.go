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
	"k8s.io/api/policy/v1beta1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/syncers"
)

type pdbSyncer struct {
	cluster *api.MysqlCluster
}

// NewPDBSyncer returns the syncer for pdb
func NewPDBSyncer(cluster *api.MysqlCluster) syncers.Interface {
	return &pdbSyncer{
		cluster: cluster,
	}
}

func (s *pdbSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &v1beta1.PodDisruptionBudget{
		Name:      s.cluster.GetNameForResource(api.PodDisruptionBudget),
		Namespace: s.cluster.Namespace,
	}
}

func (s *pdbSyncer) ShouldHaveOwnerReference() {
	return true
}

func (s *pdbSyncer) Sync(in runtime.Object) error {
	out := in.(*v1beta1.PodDisruptionBudget)
	if out.Spec.MinAvailable != nil {
		// this mean that pdb is created and should return because spec is imutable
		return nil
	}
	out.Spec.MinAvailable = s.cluster.Spec.MinAvailable
}