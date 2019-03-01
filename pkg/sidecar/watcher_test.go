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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	reconcileTime = 100 * time.Millisecond
)

var _ = Describe("Secret watcher", func() {
	var (
		stop chan struct{}
		cfg  *Config
		loop *reconcileLoop
	)

	BeforeEach(func() {
		stop = make(chan struct{})
		cfg = &Config{
			Hostname:       "cluster-mysql-0",
			ClusterName:    "cluster",
			Namespace:      "default",
			ServiceName:    "cluster-mysql-nodes",
			BackupUser:     "backup-user",
			BackupPassword: "backup-password",

			ReconcileTime: reconcileTime,
		}

		// create reconcile loop(Subject) and start it
		loop = NewSubject(cfg).(*reconcileLoop)
		loop.Start(stop)
	})

	AfterEach(func() {
		close(stop)
	})

	It("should add new observer and run it", func() {
		name := "test-obs"
		run := make(chan struct{}, 1)
		loop.AddObserver(name, func(cfg *Config) error {
			run <- struct{}{}
			return nil
		})

		// first run when added
		Eventually(run).Should(Receive())
		// second run when reconcile time is expired
		Eventually(run).Should(Receive())

		_, ok := loop.observers.Load(name)
		Expect(ok).To(Equal(true))
	})
})
