package mysqlcluster

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *cluster) createHeadlessService() apiv1.Service {
	return apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(HeadlessSVC),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{apiv1.ServicePort{
				Name:       "mysql",
				Port:       MysqlPort,
				TargetPort: TargetPort,
				Protocol:   "TCP",
			}},
			Selector:  c.getLabels(map[string]string{}),
			ClusterIP: "None",
		},
	}
}
