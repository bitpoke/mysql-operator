package v1

import (
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchEndpoints(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Endpoints) *core.Endpoints) (*core.Endpoints, kutil.VerbType, error) {
	cur, err := c.CoreV1().Endpoints(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Endpoints %s/%s.", meta.Namespace, meta.Name)
		out, err := c.CoreV1().Endpoints(meta.Namespace).Create(transform(&core.Endpoints{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Endpoints",
				APIVersion: core.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchEndpoints(c, cur, transform)
}

func PatchEndpoints(c kubernetes.Interface, cur *core.Endpoints, transform func(*core.Endpoints) *core.Endpoints) (*core.Endpoints, kutil.VerbType, error) {
	return PatchEndpointsObject(c, cur, transform(cur.DeepCopy()))
}

func PatchEndpointsObject(c kubernetes.Interface, cur, mod *core.Endpoints) (*core.Endpoints, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, core.Endpoints{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Endpoints %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.CoreV1().Endpoints(cur.Namespace).Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}
