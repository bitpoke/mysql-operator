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
	"k8s.io/apimachinery/pkg/runtime"
)

// EventReason is a kubernetes event reason
// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
// to automate handling of events, so imagine people writing switch statements to handle them.
type EventReason string

func (e *EventReason) String() string {
	return string(*e)
}

// Interface represents syncer. A syncer knows how to mutate a given object
// (known as subject), in the context of controller-runtime's CreateOrUpdate and
// also has all the data for setting the controller reference and emitting
// kubernetes events
type Interface interface {
	// GetObject returns the kubernetes object for which sync applies
	GetObject() runtime.Object
	// GetOwner returns the object owner or nil if object does not have one
	GetOwner() runtime.Object
	// Sync is a function which mutates the existing object toward the desired state
	SyncFn(existing runtime.Object) error
	// GetEventReasonForError returns a kubernetes event reason for a given error
	GetEventReasonForError(error) EventReason
}
