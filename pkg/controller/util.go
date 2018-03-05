package controller

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	// KeyFunc
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// QueuingEventHandler is an implementation of cache.ResourceEventHandler that
// simply queues objects that are added/updated/deleted.
type QueuingEventHandler struct {
	Queue workqueue.RateLimitingInterface
}

// Enqueue puts object to queue
func (q *QueuingEventHandler) Enqueue(obj interface{}) {
	key, err := KeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	q.Queue.Add(key)
}

// OnAdd is the method that enqueue obj
func (q *QueuingEventHandler) OnAdd(obj interface{}) {
	q.Enqueue(obj)
}

// OnUpdate enqueue new object
func (q *QueuingEventHandler) OnUpdate(old, new interface{}) {
	if reflect.DeepEqual(old, new) {
		return
	}
	q.Enqueue(new)
}

// OnDelete enqueue the deleted object
func (q *QueuingEventHandler) OnDelete(obj interface{}) {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = tombstone.Obj
	}
	q.Enqueue(obj)
}

// BlockingEventHandler is an implementation of cache.ResourceEventHandler that
// simply synchronously calls it's WorkFunc upon calls to OnAdd, OnUpdate or
// OnDelete.
type BlockingEventHandler struct {
	WorkFunc func(obj interface{})
}

// Enqueue runs WorkFunc on obj
func (b *BlockingEventHandler) Enqueue(obj interface{}) {
	b.WorkFunc(obj)
}

// OnAdd enqueue obj
func (b *BlockingEventHandler) OnAdd(obj interface{}) {
	b.WorkFunc(obj)
}

// OnUpdate enqueue new object
func (b *BlockingEventHandler) OnUpdate(old, new interface{}) {
	if reflect.DeepEqual(old, new) {
		return
	}
	b.WorkFunc(new)
}

// OnDelete enqueue deleted obj
func (b *BlockingEventHandler) OnDelete(obj interface{}) {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = tombstone.Obj
	}
	b.WorkFunc(obj)
}
