package mysqlcluster

import (
	kcore "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *cFactory) syncHeadlessService() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.getNameForResource(HeadlessSVC),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchService(f.client, meta,
		func(in *core.Service) *core.Service {
			in.Spec = core.ServiceSpec{
				Ports: []core.ServicePort{core.ServicePort{
					Name:       "mysql",
					Port:       MysqlPort,
					TargetPort: TargetPort,
					Protocol:   "TCP",
				}},
				Selector:  f.getLabels(map[string]string{}),
				ClusterIP: "None",
			}
			return in
		})

	state = getStatusFromKVerb(act)
	return
}
