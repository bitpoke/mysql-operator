package v1beta1

import (
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	storage "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchStorageClass(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*storage.StorageClass) *storage.StorageClass) (*storage.StorageClass, kutil.VerbType, error) {
	cur, err := c.StorageV1beta1().StorageClasses().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating StorageClass %s.", meta.Name)
		out, err := c.StorageV1beta1().StorageClasses().Create(transform(&storage.StorageClass{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StorageClass",
				APIVersion: storage.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchStorageClass(c, cur, transform)
}

func PatchStorageClass(c kubernetes.Interface, cur *storage.StorageClass, transform func(*storage.StorageClass) *storage.StorageClass) (*storage.StorageClass, kutil.VerbType, error) {
	return PatchStorageClassObject(c, cur, transform(cur.DeepCopy()))
}

func PatchStorageClassObject(c kubernetes.Interface, cur, mod *storage.StorageClass) (*storage.StorageClass, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, storage.StorageClass{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching StorageClass %s with %s.", cur.Name, string(patch))
	out, err := c.StorageV1beta1().StorageClasses().Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateStorageClass(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*storage.StorageClass) *storage.StorageClass) (result *storage.StorageClass, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.StorageV1beta1().StorageClasses().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.StorageV1beta1().StorageClasses().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update StorageClass %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update StorageClass %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}
