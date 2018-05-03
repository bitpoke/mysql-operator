package v1

import (
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchPod(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Pod) *core.Pod) (*core.Pod, kutil.VerbType, error) {
	cur, err := c.CoreV1().Pods(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Pod %s/%s.", meta.Namespace, meta.Name)
		out, err := c.CoreV1().Pods(meta.Namespace).Create(transform(&core.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: core.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchPod(c, cur, transform)
}

func PatchPod(c kubernetes.Interface, cur *core.Pod, transform func(*core.Pod) *core.Pod) (*core.Pod, kutil.VerbType, error) {
	return PatchPodObject(c, cur, transform(cur.DeepCopy()))
}

func PatchPodObject(c kubernetes.Interface, cur, mod *core.Pod) (*core.Pod, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, core.Pod{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Pod %s/%s with %s", cur.Namespace, cur.Name, string(patch))
	out, err := c.CoreV1().Pods(cur.Namespace).Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdatePod(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Pod) *core.Pod) (result *core.Pod, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.CoreV1().Pods(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.CoreV1().Pods(cur.Namespace).Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update Pod %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update Pod %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

// ref: https://github.com/coreos/prometheus-operator/blob/c79166fcff3dae7bb8bc1e6bddc81837c2d97c04/pkg/k8sutil/k8sutil.go#L64
// PodRunningAndReady returns whether a pod is running and each container has
// passed it's ready state.
func PodRunningAndReady(pod core.Pod) (bool, error) {
	switch pod.Status.Phase {
	case core.PodFailed, core.PodSucceeded:
		return false, errors.New("pod completed")
	case core.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type != core.PodReady {
				continue
			}
			return cond.Status == core.ConditionTrue, nil
		}
		return false, errors.New("pod ready condition not found")
	}
	return false, nil
}

func RestartPods(kubeClient kubernetes.Interface, namespace string, selector *metav1.LabelSelector) error {
	r, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}
	return kubeClient.CoreV1().Pods(namespace).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: r.String(),
	})
}

func WaitUntilPodRunning(kubeClient kubernetes.Interface, meta metav1.ObjectMeta) error {
	return wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		if pod, err := kubeClient.CoreV1().Pods(meta.Namespace).Get(meta.Name, metav1.GetOptions{}); err == nil {
			runningAndReady, _ := PodRunningAndReady(*pod)
			return runningAndReady, nil
		}
		return false, nil
	})
}

func WaitUntilPodRunningBySelector(kubeClient kubernetes.Interface, namespace string, selector *metav1.LabelSelector, count int) error {
	r, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}

	return wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		podList, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: r.String(),
		})
		if err != nil {
			return false, nil
		}

		if len(podList.Items) != count {
			return false, nil
		}

		for _, pod := range podList.Items {
			runningAndReady, _ := PodRunningAndReady(pod)
			if !runningAndReady {
				return false, nil
			}
		}
		return true, nil
	})
}

func WaitUntilPodDeletedBySelector(kubeClient kubernetes.Interface, namespace string, selector *metav1.LabelSelector) error {
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}

	return wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		podList, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: sel.String(),
		})
		if err != nil {
			return false, nil
		}
		return len(podList.Items) == 0, nil
	})
}

// WaitUntillPodTerminatedByLabel waits until all pods with the label are terminated. Timeout is 5 minutes.
func WaitUntillPodTerminatedByLabel(kubeClient kubernetes.Interface, namespace string, label string) error {
	return wait.PollImmediate(kutil.RetryInterval, kutil.GCTimeout, func() (bool, error) {
		podList, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			return false, nil
		}
		return len(podList.Items) == 0, nil
	})
}
