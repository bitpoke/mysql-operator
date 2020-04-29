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
	"context"
	"fmt"
	"sync"

	"github.com/onsi/gomega"

	"github.com/presslabs/mysql-operator/pkg/internal/mysql"
)

// SQLCall ...
type SQLCall func(query string, args ...interface{}) error

// SQLRunner implements a fake query runner that can be used for mocking in tests
type SQLRunner struct {
	expectedCalls   []SQLCall
	lock            sync.Mutex
	allowExtraCalls bool
	dsn             string
	expectedDSN     *string
}

// AddExpectedCalls appends a "run" function, that will be called and discarded the next time the
// query runner will be used
func (qr *SQLRunner) AddExpectedCalls(expectedCalls ...SQLCall) {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.expectedCalls = append(qr.expectedCalls, expectedCalls...)
}

// PurgeExpectedCalls removes all the expected query runner calls
func (qr *SQLRunner) PurgeExpectedCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.expectedCalls = []SQLCall{}
}

// Run implements the logic behind the fake query runner
func (qr *SQLRunner) runCall(query mysql.Query) error {
	qr.lock.Lock()
	defer qr.lock.Unlock()

	if qr.expectedDSN != nil {
		gomega.Expect(qr.dsn).To(gomega.Equal(*qr.expectedDSN), "DSN does not match")
	}

	if len(qr.expectedCalls) == 0 && qr.allowExtraCalls {
		return nil
	}

	unexpectedMessage := fmt.Sprintf(
		"No expected SQLRunner calls left, but got the following call: (dsn %s, query %s, args %s)",
		qr.dsn, query.String(), query.Args(),
	)
	gomega.Expect(qr.expectedCalls).ToNot(gomega.BeEmpty(), unexpectedMessage)
	call := qr.expectedCalls[0]
	qr.expectedCalls = qr.expectedCalls[1:]

	return call(query.String(), query.Args()...)
}

// QueryExec mock call
func (qr *SQLRunner) QueryExec(_ context.Context, query mysql.Query) error {
	return qr.runCall(query)
}

// QueryRow mock call
func (qr *SQLRunner) QueryRow(_ context.Context, query mysql.Query, _ ...interface{}) error {
	return qr.runCall(query)
}

// QueryRows mock call
func (qr *SQLRunner) QueryRows(_ context.Context, query mysql.Query) (mysql.Rows, error) {
	return nil, qr.runCall(query)
}

// AssertNoCallsLeft can be used to assert that there are no expected remaining query runner calls
func (qr *SQLRunner) AssertNoCallsLeft() {
	qr.lock.Lock()
	defer qr.lock.Unlock()

	gomega.Expect(qr.expectedCalls).To(gomega.BeEmpty())
}

// AllowExtraCalls will allow the fake query runner to be used without expecting any calls
func (qr *SQLRunner) AllowExtraCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.allowExtraCalls = true
}

// DisallowExtraCalls will disallow the fake query runner to be used without expecting any calls
func (qr *SQLRunner) DisallowExtraCalls() {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.allowExtraCalls = false
}

// AssertDSN will check that the expected DSN is set
func (qr *SQLRunner) AssertDSN(dsn string) {
	qr.lock.Lock()
	defer qr.lock.Unlock()
	qr.dsn = dsn
}

// NewQueryRunner will create a new fake.SQLRunner
func NewQueryRunner(allowExtraCalls bool) *SQLRunner {
	return &SQLRunner{
		lock:            sync.Mutex{},
		allowExtraCalls: allowExtraCalls,
	}
}

// NewFakeFactory returns a mysql.SQLRunnerFactory but with the fake SQLRunner received as parameter
func NewFakeFactory(fakeSR *SQLRunner) mysql.SQLRunnerFactory {
	return func(cfg *mysql.Config, errs ...error) (mysql.SQLRunner, func(), error) {
		if len(errs) > 0 && errs[0] != nil {
			return nil, func() {}, errs[0]
		}
		fakeSR.lock.Lock()
		defer fakeSR.lock.Unlock()
		fakeSR.dsn = cfg.GetMysqlDSN()

		return fakeSR, func() {}, nil
	}
}
