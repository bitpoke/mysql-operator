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
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/syncer"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

// NewPDBSyncer returns the syncer for pdb
func NewPDBSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster) syncer.Interface {
	obj := &policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.PodDisruptionBudget),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("PDB", cluster.Unwrap(), obj, c, scheme, func(in runtime.Object) error {
		out := in.(*policy.PodDisruptionBudget)
		if out.Spec.MinAvailable != nil {
			// this mean that pdb is created and should return because spec is imutable
			return nil
		}
		ma := intstr.FromString(cluster.Spec.MinAvailable)
		out.Spec.MinAvailable = &ma
		out.Spec.Selector = metav1.SetAsLabelSelector(cluster.GetSelectorLabels())
		return nil
	})
}
