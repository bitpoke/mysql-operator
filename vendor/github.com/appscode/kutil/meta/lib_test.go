package meta_test

import (
	"fmt"
	"testing"

	"github.com/appscode/kutil/meta"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var lblAphlict = map[string]string{
	"app": "AppAphlictserver",
}

func TestMarshalToYAML(t *testing.T) {
	obj := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "AppAphlictserver",
			Namespace: core.NamespaceDefault,
			Labels:    lblAphlict,
		},
		Spec: core.ServiceSpec{
			Selector: lblAphlict,
			Type:     core.ServiceTypeNodePort,
			Ports: []core.ServicePort{
				{
					Port:       int32(22280),
					Protocol:   core.ProtocolTCP,
					TargetPort: intstr.FromString("client-server"),
					Name:       "client-server",
				},
				{
					Port:       int32(22281),
					Protocol:   core.ProtocolTCP,
					TargetPort: intstr.FromString("admin-server"),
					Name:       "admin-server",
				},
			},
		},
	}

	b, err := meta.MarshalToYAML(obj, core.SchemeGroupVersion)
	fmt.Println(err)
	fmt.Println(string(b))
}
