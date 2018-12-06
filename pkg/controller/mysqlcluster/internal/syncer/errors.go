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

package mysqlcluster

import "fmt"

// SyncErrCode is the error code
type SyncErrCode int

const (
	// PodNotFound represents the error when pod is not found
	PodNotFound SyncErrCode = iota
)

// SyncError is the error type for pod syncer
type SyncError struct {
	syncer  string
	code    SyncErrCode
	details string
}

func (e *SyncError) Error() string {
	return fmt.Sprintf("%s(%d: %s)", e.syncer, e.code, e.details)
}

// NewError returns a syncer error
func NewError(code SyncErrCode, syncer, details string) error {
	return &SyncError{
		syncer:  syncer,
		code:    code,
		details: details,
	}
}

// NewPodNotFoundError returns a PodNotFound error
func NewPodNotFoundError() error {
	return &SyncError{
		syncer:  "PodSyncer",
		code:    PodNotFound,
		details: "pod was not found",
	}
}

// IsPodNotFound check if it's a PodNotFound error
func IsPodNotFound(err error) bool {
	switch t := err.(type) {
	default:
		return false
	case *SyncError:
		return t.code == PodNotFound
	}
}
