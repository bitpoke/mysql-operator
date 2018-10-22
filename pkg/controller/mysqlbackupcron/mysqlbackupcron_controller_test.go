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

// nolint: errcheck
package mysqlbackupcron

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	cronpkg "github.com/wgliang/cron"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const timeout = time.Second * 2

var _ = Describe("MysqlBackupCron controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		// cron job
		cron *cronpkg.Cron
	)

	BeforeEach(func() {
		var recFn reconcile.Reconciler
		cron = cronpkg.New()

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newReconciler(mgr, cron))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	// instantiate a cluster and a backup
	var (
		expectedRequest reconcile.Request
		cluster         *api.MysqlCluster
		clusterKey      types.NamespacedName
		err             error
		schedule        cronpkg.Schedule
	)

	BeforeEach(func() {
		clusterName := fmt.Sprintf("cluster-%d", rand.Int31())
		ns := "default"

		clusterKey = types.NamespacedName{Name: clusterName, Namespace: ns}
		expectedRequest = reconcile.Request{
			NamespacedName: clusterKey,
		}

		cluster = &api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   2,
				SecretName: "a-secret",

				BackupSchedule:   "* * * * *",
				BackupSecretName: "a-backup-secret",
				BackupURL:        "gs://bucket/",
			},
		}

		schedule, err = cronpkg.Parse(cluster.Spec.BackupSchedule)
		Expect(err).To(Succeed())
	})

	When("a cluster with a backup scheduler is created", func() {
		BeforeEach(func() {
			Expect(c.Create(context.TODO(), cluster)).To(Succeed())

			// Initial reconciliation
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// just when a cluster is created
			Consistently(requests, 2*time.Second).ShouldNot(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			c.Delete(context.TODO(), cluster)
		})

		It("should register the cluster into cron", func() {
			Expect(cron.Entries()).To(haveCronJob(cluster.Name, schedule))
		})

		It("should unregister if the cluster is deleted", func() {
			c.Delete(context.TODO(), cluster)

			// expect an reconcile event
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			Expect(cron.Entries()).ToNot(haveCronJob(cluster.Name, schedule))
		})

		It("should update cluster backup schedule", func() {
			// update cluster scheduler
			cluster.Spec.BackupSchedule = "0 0 * * *"
			newSchedule, _ := cronpkg.Parse(cluster.Spec.BackupSchedule)
			Expect(c.Update(context.TODO(), cluster)).To(Succeed())

			// expect an reconcile event
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// check cron entry for right scheduler
			Expect(cron.Entries()).To(haveCronJob(cluster.Name, newSchedule))
		})

		It("should be just one entry for a cluster", func() {
			// update cluster spec
			cluster.Spec.MysqlConf = map[string]string{
				"something": "new",
			}
			Expect(c.Update(context.TODO(), cluster)).To(Succeed())

			// expect an reconcile event
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// check cron entry to have a single entry
			Expect(cron.Entries()).To(HaveLen(1))
		})

		It("should update backup history limit", func() {
			// update backup history limit
			limit := 10
			cluster.Spec.BackupScheduleJobsHistoryLimit = &limit
			Expect(c.Update(context.TODO(), cluster)).To(Succeed())

			// expect an reconcile event
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// check cron entry to have a single entry
			Expect(cron.Entries()).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Job": MatchFields(IgnoreExtras, Fields{
					"Name":                           Equal(cluster.Name),
					"BackupScheduleJobsHistoryLimit": PointTo(Equal(limit)),
				}),
			}))))
		})
	})
})

func haveCronJob(name string, sched cronpkg.Schedule) gomegatypes.GomegaMatcher {
	return ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
		"Job": MatchFields(IgnoreExtras, Fields{
			"Name": Equal(name),
		}),
		"Schedule": Equal(sched),
	})))
}
