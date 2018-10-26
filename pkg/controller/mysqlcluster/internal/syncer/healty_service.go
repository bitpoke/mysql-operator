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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// NewHealthySVCSyncer returns a service syncer
func NewHealthySVCSyncer(c client.Client, scheme *runtime.Scheme, cluster *api.MysqlCluster) syncer.Interface {
	obj := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(api.HealthyNodesService),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("HealthySVC", cluster, obj, c, scheme, func(in runtime.Object) error {
		out := in.(*core.Service)

		out.Spec.Type = "ClusterIP"
		out.Spec.Selector = cluster.GetLabels()
		out.Spec.Selector["healty"] = "yes"

		if len(out.Spec.Ports) != 1 {
			out.Spec.Ports = make([]core.ServicePort, 1)
		}
		out.Spec.Ports[0].Name = MysqlPortName
		out.Spec.Ports[0].Port = MysqlPort
		out.Spec.Ports[0].TargetPort = TargetPort
		out.Spec.Ports[0].Protocol = core.ProtocolTCP

		return nil
	})
}
