/*
Copyright 2019 Pressinfra SRL.

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

package meta

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddFinalizer add a finalizer in ObjectMeta
func AddFinalizer(meta *metav1.ObjectMeta, finalizer string) {
	if !HasFinalizer(meta, finalizer) {
		meta.Finalizers = append(meta.Finalizers, finalizer)
	}
}

// HasFinalizer returns true if ObjectMeta has the finalizer
func HasFinalizer(meta *metav1.ObjectMeta, finalizer string) bool {
	return containsString(meta.Finalizers, finalizer)
}

// RemoveFinalizer removes the finalizer from ObjectMeta
func RemoveFinalizer(meta *metav1.ObjectMeta, finalizer string) {
	meta.Finalizers = removeString(meta.Finalizers, finalizer)
}

// containsString is a helper functions to check string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString is a helper functions to remove string from a slice of strings.
func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}
