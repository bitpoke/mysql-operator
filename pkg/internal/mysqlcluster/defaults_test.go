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

package mysqlcluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("MySQL defaults", func() {
	It("humanize should not round the value", func() {
		q := resource.MustParse("1.5Gi")
		hq := humanizeSize(q.Value())
		Expect(hq.String()).To(Equal("1536M"))

		q2 := resource.MustParse("321Mi")
		hq2 := humanizeSize(q2.Value())
		Expect(hq2.String()).To(Equal("321M"))

		q3 := resource.MustParse("1.07Gi")
		hq3 := humanizeSize(q3.Value())
		Expect(hq3.String()).To(Equal("1095M"))

		q4 := resource.MustParse("1200Ki")
		hq4 := humanizeSize(q4.Value())
		Expect(hq4.String()).To(Equal("1M"))
	})
})
