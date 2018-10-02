/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllerref

import (
	"fmt"

	"github.com/appscode/go/types"
	dynamicclientset "github.com/appscode/kutil/dynamic/clientset"
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	k8s "k8s.io/kubernetes/pkg/controller"
)

type UnstructuredManager struct {
	k8s.BaseControllerRefManager
	parentKind schema.GroupVersionKind
	childKind  schema.GroupVersionKind
	client     *dynamicclientset.ResourceClient
}

func NewUnstructuredManager(client *dynamicclientset.ResourceClient, parent metav1.Object, selector labels.Selector, parentKind, childKind schema.GroupVersionKind, canAdopt func() error) *UnstructuredManager {
	return &UnstructuredManager{
		BaseControllerRefManager: k8s.BaseControllerRefManager{
			Controller:   parent,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		parentKind: parentKind,
		childKind:  childKind,
		client:     client,
	}
}

func (m *UnstructuredManager) ClaimChildren(children []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	var claimed []*unstructured.Unstructured
	var errlist []error

	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.adoptChild(obj.(*unstructured.Unstructured))
	}
	release := func(obj metav1.Object) error {
		return m.releaseChild(obj.(*unstructured.Unstructured))
	}

	for _, child := range children {
		ok, err := m.ClaimObject(child, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, child)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

func (m *UnstructuredManager) adoptChild(obj *unstructured.Unstructured) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt %v %v/%v (%v): %v", m.childKind.Kind, obj.GetNamespace(), obj.GetName(), obj.GetUID(), err)
	}
	glog.Infof("%v %v/%v: adopting %v %v", m.parentKind.Kind, m.Controller.GetNamespace(), m.Controller.GetName(), m.childKind.Kind, obj.GetName())
	controllerRef := metav1.OwnerReference{
		APIVersion:         m.parentKind.GroupVersion().String(),
		Kind:               m.parentKind.Kind,
		Name:               m.Controller.GetName(),
		UID:                m.Controller.GetUID(),
		Controller:         types.TrueP(),
		BlockOwnerDeletion: types.TrueP(),
	}

	// We can't use strategic merge patch because we want this to work with custom resources.
	// We can't use merge patch because that would replace the whole list.
	// We can't use JSON patch ops because that wouldn't be idempotent.
	// The only option is GET/PUT with ResourceVersion.
	_, err := m.client.UpdateWithRetries(obj, func(obj *unstructured.Unstructured) bool {
		ownerRefs := addOwnerReference(obj.GetOwnerReferences(), controllerRef)
		obj.SetOwnerReferences(ownerRefs)
		return true
	})
	return err
}

func (m *UnstructuredManager) releaseChild(obj *unstructured.Unstructured) error {
	glog.Infof("%v %v/%v: releasing %v %v", m.parentKind.Kind, m.Controller.GetNamespace(), m.Controller.GetName(), m.childKind.Kind, obj.GetName())
	_, err := m.client.UpdateWithRetries(obj, func(obj *unstructured.Unstructured) bool {
		ownerRefs := removeOwnerReference(obj.GetOwnerReferences(), m.Controller.GetUID())
		obj.SetOwnerReferences(ownerRefs)
		return true
	})
	if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
		// If the original object is gone, that's fine because we're giving up on this child anyway.
		return nil
	}
	return err
}
