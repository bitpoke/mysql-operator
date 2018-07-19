/*
Copyright 2018 Google Inc.

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

package informer

import (
	"fmt"
	"sync"
	"time"

	dynamicclientset "github.com/appscode/kutil/dynamic/clientset"
	dynamiclister "github.com/appscode/kutil/dynamic/lister"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// SharedIndexInformer is an extension of the standard interface of the same
// name, adding the ability to remove event handlers that you added.
type SharedIndexInformer interface {
	cache.SharedIndexInformer

	// RemoveEventHandlers removes all event handlers added through this instance
	// of SharedIndexInformer. It does not affect other handlers on the same
	// underlying shared informer.
	//
	// This is necessary because the underlying shared informer may continue
	// running if others are using it, so you should remove the handlers you added
	// when you're no longer interested in receiving events.
	RemoveEventHandlers()
}

// ResourceInformer represents a "subscription" to a shared informer and lister.
//
// Users of this package shouldn't create ResourceInformers directly.
// The SharedInformerFactory returns a new ResourceInformer for each request,
// but multiple ResourceInformers may share the same underlying informer if they
// are for the same apiVersion and resource.
//
// When you're done with a ResourceInformer, you should call Close() on it.
// Once all ResourceInformers for a shared informer are closed, the shared
// informer is stopped.
type ResourceInformer struct {
	sharedResourceInformer *sharedResourceInformer
	informerWrapper        *informerWrapper
}

func newResourceInformer(sri *sharedResourceInformer) *ResourceInformer {
	return &ResourceInformer{
		sharedResourceInformer: sri,
		informerWrapper: &informerWrapper{
			SharedIndexInformer:    sri.informer,
			sharedResourceInformer: sri,
		},
	}
}

// Informer returns an interface to the dynamic, shared informer.
// If you add any event handlers with this interface, you should arrange to call
// RemoveEventHandlers() when you want to stop receiving events.
func (ri *ResourceInformer) Informer() SharedIndexInformer {
	return ri.informerWrapper
}

// Lister returns a shared, dynamic lister that's analogous to the static
// listers generated for static types.
func (ri *ResourceInformer) Lister() *dynamiclister.Lister {
	return ri.sharedResourceInformer.lister
}

// Close marks this ResourceInformer as unused, allowing the underlying shared
// informer to be stopped when no users are left.
// You should call this when you no longer need the informer, so the watches
// and relists can be stopped.
func (ri *ResourceInformer) Close() {
	// Decrement the reference count for the sharedResourceInformer.
	ri.sharedResourceInformer.close()
}

// sharedResourceInformer is the actual, single informer that's shared by
// multiple ResourceInformer instances.
type sharedResourceInformer struct {
	informer cache.SharedIndexInformer
	lister   *dynamiclister.Lister

	defaultResyncPeriod time.Duration

	eventHandlers *sharedEventHandler

	close func()
}

func newSharedResourceInformer(client *dynamicclientset.ResourceClient, defaultResyncPeriod time.Duration, close func()) *sharedResourceInformer {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  client.List,
			WatchFunc: client.Watch,
		},
		&unstructured.Unstructured{},
		defaultResyncPeriod,
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)
	sri := &sharedResourceInformer{
		close:               close,
		informer:            informer,
		defaultResyncPeriod: defaultResyncPeriod,

		lister: dynamiclister.New(client.GroupResource(), informer.GetIndexer()),
	}
	sri.eventHandlers = newSharedEventHandler(sri.lister, defaultResyncPeriod)
	informer.AddEventHandler(sri.eventHandlers)
	return sri
}

// sharedEventHandler is the one and only event handler that's actually added
// to the shared informer. All other event handlers are actually only added here
// and then this handler broadcasts to them.
//
// We do this because the real informer library has no support for removing
// event handlers. Unlike most users of that library, we need to dynamically add
// and remove handlers throughout the lifetime of a shared informer.
type sharedEventHandler struct {
	lister       *dynamiclister.Lister
	relistPeriod time.Duration

	mutex    sync.RWMutex
	handlers map[*informerWrapper][]*eventHandler
}

func newSharedEventHandler(lister *dynamiclister.Lister, relistPeriod time.Duration) *sharedEventHandler {
	return &sharedEventHandler{
		lister:       lister,
		relistPeriod: relistPeriod,
		handlers:     make(map[*informerWrapper][]*eventHandler),
	}
}

// addHandler adds a subscriber, remembering which informerWrapper it came from.
// This lets us easily remove all handlers added through the same source.
func (seh *sharedEventHandler) addHandler(iw *informerWrapper, handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	seh.mutex.Lock()
	defer seh.mutex.Unlock()

	eh := &eventHandler{ResourceEventHandler: handler, sharedEventHandler: seh}
	seh.handlers[iw] = append(seh.handlers[iw], eh)

	// Do an initial resync to pick up anything that already exists.
	// Usually the shared informer does this when a new handler is added.
	// Since we aren't actually adding this handler to the shared informer,
	// we need to do it ourselves.
	eh.resync()

	// If the requested resync period is more frequent than the underlying relist,
	// start a timer just for this handler.
	if resyncPeriod < seh.relistPeriod {
		eh.start(resyncPeriod)
	}
}

// removeHandlers removes all handlers added through the given informerWrapper.
func (seh *sharedEventHandler) removeHandlers(iw *informerWrapper) {
	seh.mutex.Lock()
	defer seh.mutex.Unlock()

	// Stop the resync timers for these handlers, if any.
	for _, eh := range seh.handlers[iw] {
		eh.stop()
	}

	// Remove all handlers added through this informerWrapper.
	delete(seh.handlers, iw)
}

func (seh *sharedEventHandler) OnAdd(obj interface{}) {
	seh.mutex.RLock()
	defer seh.mutex.RUnlock()

	for _, handlers := range seh.handlers {
		for _, handler := range handlers {
			handler.OnAdd(obj)
		}
	}
}

func (seh *sharedEventHandler) OnUpdate(oldObj, newObj interface{}) {
	seh.mutex.RLock()
	defer seh.mutex.RUnlock()

	for _, handlers := range seh.handlers {
		for _, handler := range handlers {
			handler.OnUpdate(oldObj, newObj)
		}
	}
}

func (seh *sharedEventHandler) OnDelete(obj interface{}) {
	seh.mutex.RLock()
	defer seh.mutex.RUnlock()

	for _, handlers := range seh.handlers {
		for _, handler := range handlers {
			handler.OnDelete(obj)
		}
	}
}

// eventHandler is a single entry in the sharedEventHandler's map.
type eventHandler struct {
	cache.ResourceEventHandler

	stopCh, doneCh     chan struct{}
	sharedEventHandler *sharedEventHandler
}

// start can be optionally called to give this handler its own custom resync
// period, which can be less than the underlying shared informer's period.
func (eh *eventHandler) start(resyncPeriod time.Duration) {
	eh.stopCh = make(chan struct{})
	eh.doneCh = make(chan struct{})

	go func() {
		defer close(eh.doneCh)

		ticker := time.NewTicker(resyncPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-eh.stopCh:
				return
			case <-ticker.C:
				eh.resync()
			}
		}
	}()
}

// resync sends all the latest cached values to the handler.
//
// Unlike the underlying shared informer's resync, this does not imply a relist
// (fetching a full list from the server and restarting watches).
// That makes this useful for controllers that may want to resync periodically
// just to observe time passing, but are not actually concerned about the
// reliability of the watches keeping the local cache up-to-date.
func (eh *eventHandler) resync() {
	// List everything from the cache (not from the server).
	list, err := eh.sharedEventHandler.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("can't list for resync: %v", err))
		return
	}

	// Send them all to OnUpdate to trigger a sync for every object.
	for _, obj := range list {
		// Pass the same object for old/new to indicate a resync.
		eh.OnUpdate(obj, obj)
	}
}

func (eh *eventHandler) stop() {
	if eh.stopCh != nil {
		close(eh.stopCh)
		<-eh.doneCh
	}
}

// informerWrapper wraps a cache.SharedIndexInformer and adds the ability to
// remove handlers, turning it into the custom SharedIndexInformer interface
// defined in this package.
type informerWrapper struct {
	cache.SharedIndexInformer
	sharedResourceInformer *sharedResourceInformer
}

func (iw *informerWrapper) AddEventHandler(handler cache.ResourceEventHandler) {
	iw.sharedResourceInformer.eventHandlers.addHandler(iw, handler, iw.sharedResourceInformer.defaultResyncPeriod)
}

func (iw *informerWrapper) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	iw.sharedResourceInformer.eventHandlers.addHandler(iw, handler, resyncPeriod)
}

func (iw *informerWrapper) RemoveEventHandlers() {
	iw.sharedResourceInformer.eventHandlers.removeHandlers(iw)
}
