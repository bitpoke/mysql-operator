package v1

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/kutil"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func CreateOrPatchNode(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Node) *core.Node) (*core.Node, kutil.VerbType, error) {
	cur, err := c.CoreV1().Nodes().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Node %s/%s.", meta.Namespace, meta.Name)
		out, err := c.CoreV1().Nodes().Create(transform(&core.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: core.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchNode(c, cur, transform)
}

func PatchNode(c kubernetes.Interface, cur *core.Node, transform func(*core.Node) *core.Node) (*core.Node, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(transform(cur.DeepCopy()))
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, core.Node{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Node %s with %s", cur.Name, string(patch))
	out, err := c.CoreV1().Nodes().Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateNode(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Node) *core.Node) (result *core.Node, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.CoreV1().Nodes().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.CoreV1().Nodes().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update Node %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update Node %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}

// NodeReady returns whether a node is ready.
func NodeReady(node core.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != core.NodeReady {
			continue
		}
		return cond.Status == core.ConditionTrue
	}
	return false
}

// IsMaster returns whether a node is a master.
func IsMaster(node core.Node) bool {
	_, ok17 := node.Labels["node-role.kubernetes.io/master"]
	role16, ok16 := node.Labels["kubernetes.io/role"]
	return ok17 || (ok16 && role16 == "master")
}
