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

package cluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

var _ = Describe("Mysql cluster tests", func() {
	f := framework.NewFramework("clusters")
	It("create a cluster", func() {
		Expect(f).To(Equal(true))
	})
})
