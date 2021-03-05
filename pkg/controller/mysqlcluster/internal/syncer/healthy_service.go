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
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

// NewHealthySVCSyncer returns a service syncer
func NewHealthySVCSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster) syncer.Interface {
	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.HealthyNodesService),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("HealthySVC", cluster.Unwrap(), service, c, func() error {
		// set service labels
		service.Labels = cluster.GetLabels()
		service.Labels["mysql.presslabs.org/service-type"] = "ready-nodes"

		service.Spec.Type = "ClusterIP"
		service.Spec.Selector = cluster.GetSelectorLabels()
		service.Spec.Selector["healthy"] = "yes"

		if len(service.Spec.Ports) != 1 {
			service.Spec.Ports = make([]core.ServicePort, 1)
		}
		service.Spec.Ports[0].Name = MysqlPortName
		service.Spec.Ports[0].Port = MysqlPort
		service.Spec.Ports[0].TargetPort = TargetPort
		service.Spec.Ports[0].Protocol = core.ProtocolTCP

		return nil
	})
}
