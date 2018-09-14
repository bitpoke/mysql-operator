/*
Copyright 2018 Pressinfra SRL.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package syncer

import (
	"context"
	"fmt"

	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("syncer")

const (
	eventNormal  = "Normal"
	eventWarning = "Warning"
)

// Given an Interface, returns a controllerutil.MutateFn which also sets the
// owner reference if the subject has one
func syncFn(syncer Interface, scheme *runtime.Scheme) controllerutil.MutateFn {
	owner := syncer.GetOwner()
	return func(existing runtime.Object) error {
		err := syncer.SyncFn(existing)
		if err != nil {
			return err
		}
		if owner != nil {
			existingMeta, ok := existing.(metav1.Object)
			if !ok {
				return fmt.Errorf("%T is not a metav1.Object", existing)
			}
			ownerMeta, ok := owner.(metav1.Object)
			if !ok {
				return fmt.Errorf("%T is not a metav1.Object", owner)
			}
			err := controllerutil.SetControllerReference(ownerMeta, existingMeta, scheme)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// Sync mutates the subject of the syncer interface using controller-runtime
// CreateOrUpdate method. It takes care of setting owner references and
// recording kubernetes events where appropriate
func Sync(ctx context.Context, syncer Interface, c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) error {
	obj := syncer.GetObject()
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return fmt.Errorf("%T is not a metav1.Object", obj)
	}
	key := types.NamespacedName{Name: objMeta.GetName(), Namespace: objMeta.GetNamespace()}
	op, err := controllerutil.CreateOrUpdate(ctx, c, obj, syncFn(syncer, scheme))

	owner := syncer.GetOwner()
	if recorder != nil && owner != nil {
		reason := string(syncer.GetEventReasonForError(err))
		if err != nil {
			recorder.Eventf(owner, eventWarning, reason, "%T %s failed syncing: %s", obj, key, err)
		}
		if op != controllerutil.OperationResultNone {
			recorder.Eventf(owner, eventNormal, reason, "%T %s %s successfully", obj, key, op)
		}
	}

	log.Info(string(op), "key", key, "kind", obj.GetObjectKind().GroupVersionKind().Kind)

	return err
}

// WithoutOwner partially implements implements the syncer interface for the case the subject has no owner
type WithoutOwner struct{}

// GetOwner implementation of syncer interface for the case the subject has no owner
func (*WithoutOwner) GetOwner() runtime.Object {
	return nil
}

// BasicEventReason is the basic use case for GetEventReasonForError. It just
// returns "ObjectSyncFailed" or "ObjectSyncSuccessfull"
func BasicEventReason(objKindName string, err error) EventReason {
	if err != nil {
		return EventReason(fmt.Sprintf("%sSyncFailed", objKindName))
	}
	return EventReason(fmt.Sprintf("%sSyncSuccessfull", objKindName))
}

type syncer struct {
	name   string
	owner  runtime.Object
	obj    runtime.Object
	syncFn controllerutil.MutateFn
}

func (s *syncer) GetObject() runtime.Object { return s.obj }
func (s *syncer) GetOwner() runtime.Object  { return s.owner }
func (s *syncer) GetEventReasonForError(err error) EventReason {
	return BasicEventReason(strcase.ToCamel(s.name), err)
}
func (s *syncer) SyncFn(existing runtime.Object) error {
	return s.syncFn(existing)
}

// New creates a new syncer for a given object with an owner
// The name is used for logging and event emitting purposes and should be an
// valid go identifier in upper camel case. (eg. MysqlStatefulSet)
func New(name string, owner, obj runtime.Object, syncFn controllerutil.MutateFn) Interface {
	return &syncer{
		name:   name,
		owner:  owner,
		obj:    obj,
		syncFn: syncFn,
	}
}
