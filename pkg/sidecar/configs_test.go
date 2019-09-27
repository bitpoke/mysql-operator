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

package sidecar

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test sidecar configs", func() {
	var (
		cfg *Config
	)

	BeforeEach(func() {
		cfg = &Config{
			Hostname:       "cluster-mysql-0",
			ClusterName:    "cluster",
			Namespace:      "default",
			ServiceName:    "mysql",
			BackupUser:     "backup-user",
			BackupPassword: "backup-password",
		}
	})

	It("should fill the server id", func() {
		Expect(cfg.ServerID()).To(Equal(MysqlServerIDOffset))

		cfg.Hostname = "cluster-mysql-3"
		Expect(cfg.ServerID()).To(Equal(MysqlServerIDOffset + 3))

		cfg.MyServerIDOffset = 200
		Expect(cfg.ServerID()).To(Equal(203))
	})

	It("should get the default master", func() {
		Expect(cfg.MasterFQDN()).To(Equal("cluster-mysql-master"))
	})

	It("should determine the host ip", func() {
		Expect(retryLookupHost("localhost")).To(ContainElement("127.0.0.1"))
	})
})
