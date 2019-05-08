/*
Copyright 2019 Pressinfra SRL

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

package node

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeSQLRunner struct{}

// test if fakeer implements interface
var _ SQLInterface = &fakeSQLRunner{}

func (f *fakeSQLRunner) Wait(ctx context.Context) error {
	return nil
}

func (f *fakeSQLRunner) DisableSuperReadOnly(ctx context.Context) (func(), error) {
	return func() {}, nil
}

func (f *fakeSQLRunner) ChangeMasterTo(ctx context.Context, host, user, pass string) error {
	return nil
}

func (f *fakeSQLRunner) MarkConfigurationDone(ctx context.Context) error {
	return nil
}

func (f *fakeSQLRunner) Host() string {
	return ""
}

func (f *fakeSQLRunner) SetPurgedGTID(ctx context.Context) error {
	return nil
}

var _ = Describe("SQL functions", func() {
	It("should find not found error", func() {
		err := fmt.Errorf("Error 1146: Table 'a.a' doesn't exist")
		Expect(isMySQLError(err, 1146)).To(Equal(true))
		Expect(isMySQLError(err, 1145)).To(Equal(false))
	})
})
