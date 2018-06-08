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
	"strings"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Set K8S labels indicating role pods: Master or Replica
func (f *cFactory) updatePodLabels(masterHost string) error {
	for _, ns := range f.cluster.Status.Nodes {
		pod, err := getPodForHostname(f.client, f.namespace, f.getLabels(map[string]string{}), ns.Name)
		if err != nil {
			glog.Errorf("Failed to update pod labels: %s", err)
			continue
		}

		labels := pod.GetLabels()
		val, desiredVal := "replica", "replica"
		exists := false
		val, exists = labels["role"]

		if strings.Contains(masterHost, pod.Name) {
			desiredVal = "master"
		}

		if !exists || val != desiredVal {
			labels["role"] = desiredVal
			glog.Infof("Updating labels for Pod: %s", pod.Name)

			meta := metav1.ObjectMeta{
				Name:            pod.Name,
				Labels:          labels,
				OwnerReferences: pod.GetOwnerReferences(),
				Namespace:       pod.GetNamespace(),
			}
			kcore.CreateOrPatchPod(f.client, meta,
				func(in *core.Pod) *core.Pod {
					in.Labels = labels
					return in
				})
		}
	}

	return nil
}
