package doctor

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// k8s-app=kube-proxy

func (d *Doctor) findKubeProxyPods() ([]core.Pod, error) {
	pods, err := d.kc.CoreV1().Pods(metav1.NamespaceSystem).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"k8s-app": "kube-proxy",
		}).String(),
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}
