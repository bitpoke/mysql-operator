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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("syncer")

const (
	eventNormal  = "Normal"
	eventWarning = "Warning"
)

func getKey(obj runtime.Object) (types.NamespacedName, error) {
	key := types.NamespacedName{}
	objMeta, ok := obj.(metav1.Object)
	if !ok {
		return key, fmt.Errorf("%T is not a metav1.Object", obj)
	}

	key.Name = objMeta.GetName()
	key.Namespace = objMeta.GetNamespace()
	return key, nil
}

func basicEventReason(objKindName string, err error) string {
	if err != nil {
		return fmt.Sprintf("%sSyncFailed", strcase.ToCamel(objKindName))
	}
	return fmt.Sprintf("%sSyncSuccessfull", strcase.ToCamel(objKindName))
}

// Sync mutates the subject of the syncer interface using controller-runtime
// CreateOrUpdate method, when obj is not nil. It takes care of setting owner
// references and recording kubernetes events where appropriate
func Sync(ctx context.Context, syncer Interface, recorder record.EventRecorder) error {
	result, err := syncer.Sync(ctx)
	owner := syncer.GetOwner()

	if recorder != nil && owner != nil && result.EventType != "" && result.EventReason != "" && result.EventMessage != "" {
		if err != nil || result.Operation != controllerutil.OperationResultNone {
			recorder.Eventf(owner, result.EventType, result.EventReason, result.EventMessage)
		}
	}

	return err
}

// WithoutOwner partially implements implements the syncer interface for the case the subject has no owner
type WithoutOwner struct{}

// GetOwner implementation of syncer interface for the case the subject has no owner
func (*WithoutOwner) GetOwner() runtime.Object {
	return nil
}
