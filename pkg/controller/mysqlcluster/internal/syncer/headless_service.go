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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/syncer"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

// NewHeadlessSVCSyncer returns a service syncer
func NewHeadlessSVCSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster) syncer.Interface {
	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.HeadlessSVC),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("HeadlessSVC", nil, service, c, scheme, func() error {
		// add general labels to this service
		service.Labels = map[string]string{
			"app.kubernetes.io/name":       "mysql",
			"app.kubernetes.io/managed-by": "mysql.presslabs.org",
		}
		service.Labels["mysql.presslabs.org/service-type"] = "namespace-nodes"

		service.Spec.ClusterIP = "None"
		service.Spec.Selector = labels.Set{
			"app.kubernetes.io/name":       "mysql",
			"app.kubernetes.io/managed-by": "mysql.presslabs.org",
		}
		// we want to be able to access pods even if the pod is not ready because the operator should update
		// the in memory table to mark the pod ready.
		service.Spec.PublishNotReadyAddresses = true

		if len(service.Spec.Ports) != 2 {
			service.Spec.Ports = make([]core.ServicePort, 2)
		}
		service.Spec.Ports[0].Name = MysqlPortName
		service.Spec.Ports[0].Port = MysqlPort
		service.Spec.Ports[0].TargetPort = TargetPort
		service.Spec.Ports[0].Protocol = core.ProtocolTCP

		service.Spec.Ports[1].Name = ExporterPortName
		service.Spec.Ports[1].Port = ExporterPort
		service.Spec.Ports[1].TargetPort = ExporterTargetPort
		service.Spec.Ports[1].Protocol = core.ProtocolTCP

		return nil
	})
}
