package v1beta1

import (
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchPodSecurityPolicy(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*policy.PodSecurityPolicy) *policy.PodSecurityPolicy) (*policy.PodSecurityPolicy, kutil.VerbType, error) {
	cur, err := c.PolicyV1beta1().PodSecurityPolicies().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating PodSecurityPolicy %s/%s.", meta.Namespace, meta.Name)
		out, err := c.PolicyV1beta1().PodSecurityPolicies().Create(transform(&policy.PodSecurityPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PodSecurityPolicy",
				APIVersion: policy.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchPodSecurityPolicy(c, cur, transform)
}

func PatchPodSecurityPolicy(c kubernetes.Interface, cur *policy.PodSecurityPolicy, transform func(*policy.PodSecurityPolicy) *policy.PodSecurityPolicy) (*policy.PodSecurityPolicy, kutil.VerbType, error) {
	return PatchPodSecurityPolicyObject(c, cur, transform(cur.DeepCopy()))
}

func PatchPodSecurityPolicyObject(c kubernetes.Interface, cur, mod *policy.PodSecurityPolicy) (*policy.PodSecurityPolicy, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, policy.PodSecurityPolicy{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching PodSecurityPolicy %s with %s.", cur.Name, string(patch))
	out, err := c.PolicyV1beta1().PodSecurityPolicies().Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdatePodSecurityPolicy(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*policy.PodSecurityPolicy) *policy.PodSecurityPolicy) (result *policy.PodSecurityPolicy, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.PolicyV1beta1().PodSecurityPolicies().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.PolicyV1beta1().PodSecurityPolicies().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update PodSecurityPolicy %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update PodSecurityPolicy %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}
