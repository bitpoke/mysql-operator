/*
Copyright 2020 Pressinfra SRL

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

package fake

import (
	"fmt"
	"sync"

	"github.com/onsi/gomega"

	"github.com/presslabs/mysql-operator/pkg/internal/mysql"
)

// QueryRunner implements a fake query runner that can be used for mocking in tests
type QueryRunner struct {
	expectedCalls   []mysql.QueryRunner
	lock            sync.Mutex
	allowExtraCalls bool
}

// AddExpectedCalls appends a "run" function, that will be called and discarded the next time the
// query runner will be used
func (qr *QueryRunner) AddExpectedCalls(expectedCalls ...mysql.QueryRunner) {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.expectedCalls = append(qr.expectedCalls, expectedCalls...)
}

// PurgeExpectedCalls removes all the expected query runner calls
func (qr *QueryRunner) PurgeExpectedCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.expectedCalls = []mysql.QueryRunner{}
}

// Run implements the logic behind the fake query runner
func (qr *QueryRunner) Run(dsn string, query string, args ...interface{}) error {
	qr.lock.Lock()
	defer qr.lock.Unlock()

	if len(qr.expectedCalls) == 0 && qr.allowExtraCalls {
		return nil
	}

	unexpectedMessage := fmt.Sprintf(
		"No expected QueryRunner calls left, but got the following call: (dsn %s, query %s, args %s)",
		dsn, query, args,
	)
	gomega.Expect(qr.expectedCalls).ToNot(gomega.BeEmpty(), unexpectedMessage)
	call := qr.expectedCalls[0]
	qr.expectedCalls = qr.expectedCalls[1:]

	return call(dsn, query, args...)
}

// AssertNoCallsLeft can be used to assert that there are no expected remaining query runner calls
func (qr *QueryRunner) AssertNoCallsLeft() {
	qr.lock.Lock()
	defer qr.lock.Unlock()

	gomega.Expect(qr.expectedCalls).To(gomega.BeEmpty())
}

// AllowExtraCalls will allow the fake query runner to be used without expecting any calls
func (qr *QueryRunner) AllowExtraCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.allowExtraCalls = true
}

// DisallowExtraCalls will disallow the fake query runner to be used without expecting any calls
func (qr *QueryRunner) DisallowExtraCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.allowExtraCalls = false
}

// NewQueryRunner returns a new fake query runner
func NewQueryRunner(allowExtraCalls bool) *QueryRunner {
	return &QueryRunner{
		lock:            sync.Mutex{},
		allowExtraCalls: allowExtraCalls,
	}
}
