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
	"github.com/presslabs/mysql-operator/pkg/syncers"
)

type headlessSVCSyncer struct {
	cluster *api.MysqlCluster
}

func NewHeadlessSVCSyncer(cluster *api.MysqlCluster) syncers.Interface {
	return &headlessSVCSyncer{
		cluster: cluster,
	}
}

func (s *headlessSVCSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &core.Service{
		Name:      s.cluster.GetNameForResource(api.HeadlessSVC),
		Namespace: s.cluster.Namespace,
	}
}

func (s *headlessSVCSyncer) ShouldHaveOwnerReference() bool {
	return true
}

func (s *headlessSVCSyncer) Sync(in runtime.Object) error {
	out := in.(*core.Service)

	out.Spec.ClusterIP = "None"
	out.Spec.Selector = f.getLabels(map[string]string{})
	if len(out.Spec.Ports) != 2 {
		out.Spec.Ports = make([]core.ServicePort, 2)
	}
	out.Spec.Ports[0].Name = MysqlPortName
	out.Spec.Ports[0].Port = MysqlPort
	out.Spec.Ports[0].TargetPort = TargetPort
	out.Spec.Ports[0].Protocol = "TCP"

	out.Spec.Ports[1].Name = ExporterPortName
	out.Spec.Ports[1].Port = ExporterPort
	out.Spec.Ports[1].TargetPort = ExporterTargetPort
	out.Spec.Ports[1].Protocol = "TCP"

	return nil

}
