package mysqlcluster

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *cFactory) createHeadlessService() apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            f.getNameForResource(HeadlessSVC),
			Labels:          f.getLabels(map[string]string{}),
			OwnerReferences: f.getOwnerReferences(),
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{apiv1.ServicePort{
				Name:       "mysql",
				Port:       MysqlPort,
				TargetPort: TargetPort,
				Protocol:   "TCP",
			}},
			Selector:  f.getLabels(map[string]string{}),
			ClusterIP: "None",
		},
	}
}
