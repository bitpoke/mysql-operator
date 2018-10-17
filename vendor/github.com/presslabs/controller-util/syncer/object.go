package syncer

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/patch"
)

// ObjectSyncer is a syncer.Interface for syncing kubernetes.Objects only by
// passing a SyncFn
type ObjectSyncer struct {
	Owner          runtime.Object
	Obj            runtime.Object
	SyncFn         controllerutil.MutateFn
	Name           string
	Client         client.Client
	Scheme         *runtime.Scheme
	previousObject runtime.Object
}

// GetObject returns the ObjectSyncer subject
func (s *ObjectSyncer) GetObject() interface{} { return s.Obj }

// GetOwner returns the ObjectSyncer owner
func (s *ObjectSyncer) GetOwner() runtime.Object { return s.Owner }

// Sync does the actual syncing and implements the syncer.Inteface Sync method
func (s *ObjectSyncer) Sync(ctx context.Context) (SyncResult, error) {
	result := SyncResult{}

	key, err := getKey(s.Obj)
	if err != nil {
		return result, err
	}

	result.Operation, err = controllerutil.CreateOrUpdate(ctx, s.Client, s.Obj, s.mutateFn())

	diff, _ := patch.NewJSONPatch(s.previousObject, s.Obj)

	if err != nil {
		result.SetEventData(eventWarning, basicEventReason(s.Name, err),
			fmt.Sprintf("%T %s failed syncing: %s", s.Obj, key, err))
		log.Error(err, string(result.Operation), "key", key, "kind", fmt.Sprintf("%T", s.Obj), "diff", diff)
	} else {
		result.SetEventData(eventNormal, basicEventReason(s.Name, err),
			fmt.Sprintf("%T %s %s successfully", s.Obj, key, result.Operation))
		log.V(1).Info(string(result.Operation), "key", key, "kind", fmt.Sprintf("%T", s.Obj), "diff", diff)
	}

	return result, err
}

// Given an ObjectSyncer, returns a controllerutil.MutateFn which also sets the
// owner reference if the subject has one
func (s *ObjectSyncer) mutateFn() controllerutil.MutateFn {
	return func(existing runtime.Object) error {
		s.previousObject = existing.DeepCopyObject()
		err := s.SyncFn(existing)
		if err != nil {
			return err
		}
		if s.Owner != nil {
			existingMeta, ok := existing.(metav1.Object)
			if !ok {
				return fmt.Errorf("%T is not a metav1.Object", existing)
			}
			ownerMeta, ok := s.Owner.(metav1.Object)
			if !ok {
				return fmt.Errorf("%T is not a metav1.Object", s.Owner)
			}
			err := controllerutil.SetControllerReference(ownerMeta, existingMeta, s.Scheme)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// NewObjectSyncer creates a new kubernetes.Object syncer for a given object
// with an owner and persists data using controller-runtime's CreateOrUpdate.
// The name is used for logging and event emitting purposes and should be an
// valid go identifier in upper camel case. (eg. MysqlStatefulSet)
func NewObjectSyncer(name string, owner, obj runtime.Object, c client.Client, scheme *runtime.Scheme, syncFn controllerutil.MutateFn) Interface {
	return &ObjectSyncer{
		Owner:  owner,
		Obj:    obj,
		SyncFn: syncFn,
		Name:   name,
		Client: c,
		Scheme: scheme,
	}
}
