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

package controllerref

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func addOwnerReference(in []metav1.OwnerReference, add metav1.OwnerReference) []metav1.OwnerReference {
	out := make([]metav1.OwnerReference, 0, len(in)+1)
	found := false
	for _, ref := range in {
		if ref.UID == add.UID {
			// We already own this. Update other fields as needed.
			out = append(out, add)
			found = true
			continue
		}
		out = append(out, ref)
	}
	if !found {
		// Add ourselves to the list.
		// Note that server-side validation is responsible for ensuring only one ControllerRef.
		out = append(out, add)
	}
	return out
}

func removeOwnerReference(in []metav1.OwnerReference, uid types.UID) []metav1.OwnerReference {
	out := make([]metav1.OwnerReference, 0, len(in))
	for _, ref := range in {
		if ref.UID != uid {
			out = append(out, ref)
		}
	}
	return out
}
