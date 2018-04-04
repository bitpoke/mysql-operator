package meta

import (
	"testing"

	"github.com/appscode/go/types"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/mergepatch"
)

func newObj() apps.Deployment {
	return apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: apps.DeploymentSpec{
			Replicas: types.Int32P(3),
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  "foo",
							Image: "foo/bar:latest",
						},
						{
							Name:  "bar",
							Image: "foo/bar:latest",
						},
					},
					Hostname: "foo-bar",
				},
			},
		},
	}
}

func getPreconditionFuncs() []mergepatch.PreconditionFunc {
	preconditions := []mergepatch.PreconditionFunc{
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
		mergepatch.RequireMetadataKeyUnchanged("namespace"),
		mergepatch.RequireKeyUnchanged("status"),
		// below methods are added in kutil/meta/patch.go
		RequireChainKeyUnchanged("spec.replicas"),
		RequireChainKeyUnchanged("spec.template.spec.containers.image"), //here container is array, yet works fine
	}
	return preconditions
}

func TestCreateStrategicPatch_Conditions(t *testing.T) {
	obj, validMod, badMod, badArrayMod := newObj(), newObj(), newObj(), newObj()
	validMod.Spec.Template.Spec.Hostname = "NewHostName"
	badMod.Spec.Replicas = types.Int32P(2)
	badArrayMod.Spec.Template.Spec.Containers[0].Image = "newImage"

	preconditions := getPreconditionFuncs()

	cases := []struct {
		name   string
		x      apps.Deployment
		y      apps.Deployment
		cond   []mergepatch.PreconditionFunc
		result bool
	}{
		{"bad modification without condition", obj, badMod, nil, true}, //	// without preconditions
		{"valid modification with condition", obj, validMod, preconditions, true},
		{"bad modification with condition", obj, badMod, preconditions, false},
		{"bad array modification with condition", obj, badArrayMod, preconditions, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CreateStrategicPatch(&c.x, &c.y, c.cond...)
			if c.result == true {
				if err != nil {
					t.Errorf("Modifications should be passed. error: %v", err)
				}
			} else if c.result == false {
				if err == nil || !mergepatch.IsPreconditionFailed(err) {
					t.Errorf("Modifications should be failed. error: %v", err)
				}
			}
		})
	}
}

func TestCreateJSONMergePatch_Conditions(t *testing.T) {
	obj, validMod, badMod, badArrayMod := newObj(), newObj(), newObj(), newObj()
	validMod.Spec.Template.Spec.Hostname = "NewHostName"
	badMod.Spec.Replicas = types.Int32P(2)
	badArrayMod.Spec.Template.Spec.Containers[0].Image = "newImage"

	preconditions := getPreconditionFuncs()

	cases := []struct {
		name   string
		x      apps.Deployment
		y      apps.Deployment
		cond   []mergepatch.PreconditionFunc
		result bool
	}{
		{"bad modification without condition", obj, badMod, nil, true}, //	// without preconditions
		{"valid modification with condition", obj, validMod, preconditions, true},
		{"bad modification with condition", obj, badMod, preconditions, false},
		{"bad array modification with condition", obj, badArrayMod, preconditions, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CreateJSONMergePatch(&c.x, &c.y, c.cond...)
			if c.result == true {
				if err != nil {
					t.Errorf("Modifications should be passed. error: %v", err)
				}
			} else if c.result == false {
				if err == nil || !mergepatch.IsPreconditionFailed(err) {
					t.Errorf("Modifications should be failed. error: %v", err)
				}
			}
		})
	}
}
