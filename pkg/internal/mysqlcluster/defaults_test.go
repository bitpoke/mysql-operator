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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("MySQL defaults unit tests", func() {
	DescribeTable("humanize should not round the value", func(quantity, expect string) {
		q := resource.MustParse(quantity)
		hq := humanizeSize(q.Value())
		Expect(hq.String()).To(Equal(expect))
	},
		Entry("fractional Gigabytes", "1.5Gi", "1536M"),
		Entry("integer Gigabytes", "2Gi", "2G"),
		Entry("overflow Megabytes to Gigabytes", "2048Mi", "2G"),
		Entry("overflow Kilobytes to Gigabytes", "2097152Ki", "2G"),
		Entry("fractional Megabytes", "1.5Mi", "1536K"),
		Entry("integer Megabytes", "128Mi", "128M"),
		Entry("overflow Kilobytes to Megabytes", "4096Ki", "4M"),
		Entry("fractional Kilobytes", "0.5Ki", "512"),
		Entry("integer Kilobytes", "1200Ki", "1200K"),
	)
})
