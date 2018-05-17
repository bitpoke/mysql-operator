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
	kcore "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func (f *cFactory) syncHeadlessService() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.HeadlessSVC),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchService(f.client, meta,
		func(in *core.Service) *core.Service {
			in.Spec.ClusterIP = "None"
			in.Spec.Selector = f.getLabels(map[string]string{})
			if len(in.Spec.Ports) != 2 {
				in.Spec.Ports = make([]core.ServicePort, 2)
			}
			in.Spec.Ports[0].Name = MysqlPortName
			in.Spec.Ports[0].Port = MysqlPort
			in.Spec.Ports[0].TargetPort = TargetPort
			in.Spec.Ports[0].Protocol = "TCP"

			in.Spec.Ports[1].Name = ExporterPortName
			in.Spec.Ports[1].Port = ExporterPort
			in.Spec.Ports[1].TargetPort = ExporterTargetPort
			in.Spec.Ports[1].Protocol = "TCP"

			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) syncMasterService() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.MasterService),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchService(f.client, meta,
		func(in *core.Service) *core.Service {
			in.Spec.ClusterIP = "None"
			if len(in.Spec.Ports) != 1 {
				in.Spec.Ports = make([]core.ServicePort, 1)
			}
			in.Spec.Ports[0].Name = MysqlPortName
			in.Spec.Ports[0].Port = MysqlPort
			in.Spec.Ports[0].TargetPort = TargetPort
			in.Spec.Ports[0].Protocol = "TCP"

			return in
		})

	state = getStatusFromKVerb(act)

	return
}

func (f *cFactory) syncHealthyNodesService() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.HealthyNodesService),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchService(f.client, meta,
		func(in *core.Service) *core.Service {
			in.Spec.ClusterIP = "None"
			if len(in.Spec.Ports) != 1 {
				in.Spec.Ports = make([]core.ServicePort, 1)
			}
			in.Spec.Ports[0].Name = MysqlPortName
			in.Spec.Ports[0].Port = MysqlPort
			in.Spec.Ports[0].TargetPort = TargetPort
			in.Spec.Ports[0].Protocol = "TCP"

			return in
		})

	state = getStatusFromKVerb(act)

	return
}
