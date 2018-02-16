package v1

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/go/types"
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	batch "k8s.io/api/batch/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchJob(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*batch.Job) *batch.Job) (*batch.Job, kutil.VerbType, error) {
	cur, err := c.BatchV1().Jobs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Job %s/%s.", meta.Namespace, meta.Name)
		out, err := c.BatchV1().Jobs(meta.Namespace).Create(transform(&batch.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: batch.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchJob(c, cur, transform)
}

func PatchJob(c kubernetes.Interface, cur *batch.Job, transform func(*batch.Job) *batch.Job) (*batch.Job, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(transform(cur.DeepCopy()))
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, batch.Job{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Job %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.BatchV1().Jobs(cur.Namespace).Patch(cur.Name, ktypes.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateJob(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*batch.Job) *batch.Job) (result *batch.Job, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.BatchV1().Jobs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.BatchV1().Jobs(cur.Namespace).Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update Job %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update Job %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

func WaitUntilJobCompletion(c kubernetes.Interface, meta metav1.ObjectMeta) error {
	return wait.PollInfinite(kutil.RetryInterval, func() (bool, error) {
		job, err := c.BatchV1().Jobs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if err != nil {
			if kerr.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}

		if job.Status.Succeeded > 0 || job.Status.Failed > types.Int32(job.Spec.BackoffLimit) {
			return true, nil
		}
		return false, nil
	})
}
