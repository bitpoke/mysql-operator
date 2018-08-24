/*
Copyright 2018 Pressinfra SRL

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

package source

import (
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// defaultTimePeriod is the time at whcich event is triggered
	defaultTimePeriod = time.Second

	// defaultBufferSize is the default number of event notifications that can be buffered.
	defaultBufferSize = 1024
)

var log = logf.Log.WithName("mysql-source")

var _ source.Source = &RecurentEventMap{}

// RecurentEventMap is used to provide a source of events originating outside the cluster
// (eh.g. GitHub Webhook callback).  RecurentEventMap requires the user to wire the external
// source (eh.g. http handler) to write GenericEvents to the underlying channel.
type RecurentEventMap struct {
	// once ensures the event distribution goroutine will be performed only once
	once sync.Once

	// lock
	lock sync.RWMutex

	// dataMap the source that will be populated by resiter methods
	dataMap map[string]event.GenericEvent

	// TimePeriod represents the period on which to send the event
	TimePeriod time.Duration

	// stop is to end ongoing goroutine, and close the channels
	stop <-chan struct{}

	// dest is the destination channels of the added event handlers
	dest []chan event.GenericEvent

	// DestBufferSize is the specified buffer size of dest channels.
	// Default to 1024 if not specified.
	DestBufferSize int

	// destLock is to ensure the destination channels are safely added/removed
	destLock sync.Mutex
}

var _ inject.Stoppable = &RecurentEventMap{}

// InjectStopChannel is internal should be called only by the Controller.
// It is used to inject the stop channel initialized by the ControllerManager.
func (rem *RecurentEventMap) InjectStopChannel(stop <-chan struct{}) error {
	if rem.stop == nil {
		rem.stop = stop
	}

	return nil
}

// Start implrements Source and should only be called by the Controller.
func (rem *RecurentEventMap) Start(
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	prct ...predicate.Predicate) error {
	// Source should have been specified by the user.

	// stop should have been injected before Start was called
	// TODO: uncomment this when is fixed
	// if rem.stop == nil {
	// 	return fmt.Errorf("must call InjectStop on RecurentEventMap before calling Start")
	// }

	// use default value if TimePeriod not specified
	if rem.TimePeriod == 0 {
		rem.TimePeriod = defaultTimePeriod
	}

	// use default value if DestBufferSize not specified
	if rem.DestBufferSize == 0 {
		rem.DestBufferSize = defaultBufferSize
	}

	if rem.dataMap == nil {
		rem.dataMap = make(map[string]event.GenericEvent)
	}

	rem.once.Do(func() {
		// Distribute GenericEvents to all EventHandler / Queue pairs Watching this source
		go rem.syncLoop()
	})

	dst := make(chan event.GenericEvent, rem.DestBufferSize)
	go func() {
		// itereate over received events for a specific handler and predicates
		for evt := range dst {
			shouldHandle := true
			for _, p := range prct {
				if !p.Generic(evt) {
					shouldHandle = false
					break
				}
			}

			if shouldHandle {
				handler.Generic(evt, queue)
			}
		}
	}()

	rem.destLock.Lock()
	defer rem.destLock.Unlock()

	rem.dest = append(rem.dest, dst)

	return nil
}

func (rem *RecurentEventMap) doStop() {
	rem.destLock.Lock()
	defer rem.destLock.Unlock()

	for _, dst := range rem.dest {
		close(dst)
	}
}

func (rem *RecurentEventMap) distribute(evt event.GenericEvent) {
	rem.destLock.Lock()
	defer rem.destLock.Unlock()

	for _, dst := range rem.dest {
		// We cannot make it under goroutine here, or we'll meet the
		// race condition of writing message to closed channels.
		// To avoid blocking, the dest channels are expected to be of
		// proper buffer size. If we still see it blocked, then
		// the controller is thought to be in an abnormal state.
		dst <- evt
	}
}

func (rem *RecurentEventMap) syncLoop() {
	for {
		select {
		case <-rem.stop:
			// Close destination channels
			rem.doStop()
			return
		case <-time.After(rem.TimePeriod):
			rem.lock.RLock()
			for _, evt := range rem.dataMap {
				rem.distribute(evt)
			}
			rem.lock.RUnlock()
		}
	}
}

func (rem *RecurentEventMap) getKey(meta metav1.Object) string {
	return fmt.Sprintf("%s/%s", meta.GetNamespace(), meta.GetName())
}

// CreateEvent is the method that register in map a new event
func (rem *RecurentEventMap) CreateEvent(evt event.CreateEvent,
	q workqueue.RateLimitingInterface) { // nolint: unparam
	if evt.Meta == nil {
		log.Error(nil, "CreateEvent received with no metadata", "CreateEvent", evt)
		return
	}

	rem.lock.Lock()
	defer rem.lock.Unlock()
	rem.dataMap[rem.getKey(evt.Meta)] = event.GenericEvent{
		Meta:   evt.Meta,
		Object: evt.Object,
	}
}

// DeleteEvent is the method that removes from list a event like this
func (rem *RecurentEventMap) DeleteEvent(evt event.DeleteEvent,
	q workqueue.RateLimitingInterface) { // nolint: unparam
	if evt.Meta == nil {
		log.Error(nil, "DeleteEvent received with no metadata", "DeleteEvent", evt)
		return
	}

	rem.lock.Lock()
	defer rem.lock.Unlock()

	_, ok := rem.dataMap[rem.getKey(evt.Meta)]
	if ok {
		delete(rem.dataMap, rem.getKey(evt.Meta))
	}
}
