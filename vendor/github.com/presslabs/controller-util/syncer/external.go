package syncer

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type externalSyncer struct {
	name   string
	obj    interface{}
	owner  runtime.Object
	syncFn func(context.Context, interface{}) (controllerutil.OperationResult, error)
}

func (s *externalSyncer) GetObject() interface{}   { return s.obj }
func (s *externalSyncer) GetOwner() runtime.Object { return s.owner }
func (s *externalSyncer) Sync(ctx context.Context) (SyncResult, error) {
	var err error
	result := SyncResult{}
	result.Operation, err = s.syncFn(ctx, s.obj)

	if err != nil {
		result.SetEventData(eventWarning, basicEventReason(s.name, err),
			fmt.Sprintf("%T failed syncing: %s", s.obj, err))
		log.Error(err, string(result.Operation), "kind", fmt.Sprintf("%T", s.obj))
	} else {
		result.SetEventData(eventNormal, basicEventReason(s.name, err),
			fmt.Sprintf("%T successfully %s", s.obj, result.Operation))
		log.V(1).Info(string(result.Operation), "kind", fmt.Sprintf("%T", s.obj))
	}

	return result, err
}

// NewExternalSyncer creates a new syncer which syncs a generic object
// persisting it's state into and external store The name is used for logging
// and event emitting purposes and should be an valid go identifier in upper
// camel case. (eg. GiteaRepo)
func NewExternalSyncer(name string, owner runtime.Object, obj interface{}, syncFn func(context.Context, interface{}) (controllerutil.OperationResult, error)) Interface {
	return &externalSyncer{
		name:   name,
		obj:    obj,
		owner:  owner,
		syncFn: syncFn,
	}
}
