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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

type headlessSVCSyncer struct {
	cluster     *api.MysqlCluster
	headlessSVC *core.Service
}

// NewHeadlessSVCSyncer returns a service syncer
func NewHeadlessSVCSyncer(cluster *api.MysqlCluster) syncer.Interface {

	obj := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(api.HeadlessSVC),
			Namespace: cluster.Namespace,
		},
	}

	return &headlessSVCSyncer{
		cluster:     cluster,
		headlessSVC: obj,
	}

}

func (s *headlessSVCSyncer) GetObject() runtime.Object { return s.headlessSVC }
func (s *headlessSVCSyncer) GetOwner() runtime.Object  { return s.cluster }
func (s *headlessSVCSyncer) GetEventReasonForError(err error) syncer.EventReason {
	return syncer.BasicEventReason("HeadlessSVC", err)
}

func (s *headlessSVCSyncer) SyncFn(in runtime.Object) error {
	out := in.(*core.Service)

	out.Spec.ClusterIP = "None"
	out.Spec.Selector = s.cluster.GetLabels()
	if len(out.Spec.Ports) != 2 {
		out.Spec.Ports = make([]core.ServicePort, 2)
	}
	out.Spec.Ports[0].Name = MysqlPortName
	out.Spec.Ports[0].Port = MysqlPort
	out.Spec.Ports[0].TargetPort = TargetPort
	out.Spec.Ports[0].Protocol = core.ProtocolTCP

	out.Spec.Ports[1].Name = ExporterPortName
	out.Spec.Ports[1].Port = api.ExporterPort
	out.Spec.Ports[1].TargetPort = ExporterTargetPort
	out.Spec.Ports[1].Protocol = core.ProtocolTCP

	return nil

}
