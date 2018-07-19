package doctor

import (
	"context"

	core_util "github.com/appscode/kutil/core/v1"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/pager"
)

func (d *Doctor) extractMasterArgs(info *ClusterInfo) error {
	pods, err := d.findMasterPods()
	if err != nil {
		return err
	}

	proxyPods, err := d.findKubeProxyPods()
	if err != nil {
		return err
	}

	var errs []error
	for _, pod := range pods {
		if c, err := d.processPod(pod); err != nil {
			errs = append(errs, err)
		} else {
			for _, proxyPod := range proxyPods {
				if proxyPod.Spec.NodeName == c.NodeName {
					c.KubeProxyFound = true
					c.KubeProxyRunning, err = core_util.PodRunningAndReady(proxyPod)
					if err != nil {
						return err
					}
				}
			}
			info.APIServers = append(info.APIServers, *c)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (d *Doctor) findMasterPods() ([]core.Pod, error) {
	pods, err := d.findMasterPodsByLabel()
	if err != nil {
		return nil, err
	}
	if len(pods) > 0 {
		return pods, nil
	}

	return d.findMasterPodsByKubernetesService()
}

func (d *Doctor) findMasterPodsByLabel() ([]core.Pod, error) {
	pods, err := d.kc.CoreV1().Pods(metav1.NamespaceSystem).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"component": "kube-apiserver",
		}).String(),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) > 0 {
		return pods.Items, nil
	}

	// kops
	pods, err = d.kc.CoreV1().Pods(metav1.NamespaceSystem).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"k8s-app": "kube-apiserver",
		}).String(),
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (d *Doctor) findMasterPodsByKubernetesService() ([]core.Pod, error) {
	ep, err := d.kc.CoreV1().Endpoints(core.NamespaceDefault).Get("kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	podIPs := sets.NewString()
	ports := sets.NewInt64()
	for _, subnet := range ep.Subsets {
		for _, addr := range subnet.Addresses {
			podIPs.Insert(addr.IP)
		}
		for _, p := range subnet.Ports {
			ports.Insert(int64(p.Port))
		}
	}

	lister := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return d.kc.CoreV1().Pods(metav1.NamespaceSystem).List(opts)
	})
	objects, err := lister.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]core.Pod, 0, podIPs.Len())
	err = meta.EachListItem(objects, func(obj runtime.Object) error {
		pod, ok := obj.(*core.Pod)
		if !ok {
			return errors.Errorf("%v is not a pod", obj)
		}
		if podIPs.Has(pod.Status.PodIP) &&
			len(pod.Spec.Containers[0].Ports) > 0 &&
			ports.Has(int64(pod.Spec.Containers[0].Ports[0].ContainerPort)) {
			pods = append(pods, *pod)
		}
		return nil
	})
	return pods, err
}
