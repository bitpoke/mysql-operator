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
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

type masterSVCSyncer struct {
	cluster *api.MysqlCluster
}

func NewMasterSVCSyncer(cluster *api.MysqlCluster) syncers.Interface {
	return &masterSVCSyncer{
		cluster: cluster,
	}
}

func (s *masterSVCSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &core.Service{
		Name:      s.cluster.GetNameForResource(api.MasterSVC),
		Namespace: s.cluster.Namespace,
	}
}

func (s *masterSVCSyncer) ShouldHaveOwnerReference() bool {
	return true
}

func (s *masterSVCSyncer) Sync(in runtime.Object) error {
	out := in.(*core.Service)

	out.Spec.Type = "ClusterIP"
	out.Spec.Selector = s.cluster.GetLabels()
	out.Spec.Selector["role"] = "master"

	if len(out.Spec.Ports) != 1 {
		out.Spec.Ports = make([]core.ServicePort, 1)
	}
	out.Spec.Ports[0].Name = MysqlPortName
	out.Spec.Ports[0].Port = MysqlPort
	out.Spec.Ports[0].TargetPort = TargetPort
	out.Spec.Ports[0].Protocol = "TCP"

	return nil

}
