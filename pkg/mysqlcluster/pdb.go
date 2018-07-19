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
	kapps "github.com/appscode/kutil/policy/v1beta1"
	"k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func (f *cFactory) syncPDB() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.StatefulSet),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	if f.cluster.Spec.MinAvailable != nil {
		_, act, err := kapps.CreateOrPatchPodDisruptionBudget(f.client, meta,
			func(in *v1beta1.PodDisruptionBudget) *v1beta1.PodDisruptionBudget {

				in.Spec.MinAvailable = f.cluster.Spec.MinAvailable
				return in
			})

		if err != nil {
			state = statusFailed
			return state, err
		}

		state = getStatusFromKVerb(act)
		return state, err
	} else {
		err = f.client.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).Delete(meta.Name, metav1.NewDeleteOptions(0))
		if kerr.IsNotFound(err) {
			state = statusDeleted
			return state, nil
		} else {
			return statusFailed, err
		}
	}
}
