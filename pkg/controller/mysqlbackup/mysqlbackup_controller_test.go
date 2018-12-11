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
package mysqlbackup

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"golang.org/x/net/context"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

const timeout = time.Second * 2

var _ = Describe("MysqlBackup controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
	)

	BeforeEach(func() {
		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newReconciler(mgr))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	// instantiate a cluster and a backup
	var (
		expectedRequest reconcile.Request
		cluster         *mysqlcluster.MysqlCluster
		backup          *mysqlbackup.MysqlBackup
		backupKey       types.NamespacedName
		jobKey          types.NamespacedName
	)

	BeforeEach(func() {
		clusterName := fmt.Sprintf("cluster-%d", rand.Int31())
		name := fmt.Sprintf("backup-%d", rand.Int31())
		ns := "default"

		backupKey = types.NamespacedName{Name: name, Namespace: ns}
		expectedRequest = reconcile.Request{
			NamespacedName: backupKey,
		}

		two := int32(2)
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   &two,
				SecretName: "a-secret",
			},
		})

		backup = mysqlbackup.New(&api.MysqlBackup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: api.MysqlBackupSpec{
				ClusterName:      clusterName,
				BackupURL:        "gs://bucket/",
				BackupSecretName: "secert",
			},
		})

		jobKey = types.NamespacedName{
			Name:      backup.GetNameForJob(),
			Namespace: backup.Namespace,
		}
	})

	When("a new mysql backup is created", func() {

		BeforeEach(func() {
			// create a cluster with 2 nodes
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())
			cluster.Status.Nodes = []api.NodeStatus{
				api.NodeStatus{
					Name:       cluster.GetPodHostname(0),
					Conditions: testutil.NodeConditions(true, false, false, false),
				},
				api.NodeStatus{
					Name:       cluster.GetPodHostname(1),
					Conditions: testutil.NodeConditions(false, true, false, true),
				},
			}
			Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())
			// create the backup
			Expect(c.Create(context.TODO(), backup.Unwrap())).To(Succeed())

			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// some extra reconcile requests may appear
			testutil.DrainChan(requests)
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
			Expect(c.Delete(context.TODO(), backup.Unwrap())).To(Succeed())
		})

		It("should have only one reconcile request", func() {
			// We need to make sure that the controller does not create infinite
			// loops
			Consistently(requests, 5*time.Second).ShouldNot(Receive(Equal(expectedRequest)))
		})

		It("should create the job", func() {
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).To(Succeed())
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("backup"))

			// should use the replica node to take the backup
			Expect(job.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
				Equal(cluster.GetPodHostname(1))))
		})

		It("should populate the defaults", func() {
			Expect(c.Get(context.TODO(), backupKey, backup.Unwrap())).To(Succeed())
			Expect(backup.Spec.BackupURL).To(ContainSubstring(mysqlbackup.BackupSuffix))
		})

		It("should update backup status as complete", func() {
			// get job
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).To(Succeed())

			// update job as completed
			job.Status.Conditions = []batch.JobCondition{
				batch.JobCondition{
					Type:   batch.JobComplete,
					Status: core.ConditionTrue,
				},
			}
			Expect(c.Status().Update(context.TODO(), job)).To(Succeed())

			// expect reqoncile request
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			Eventually(refreshFn(c, backupKey)).Should(testutil.BackupHaveCondition(api.BackupComplete, core.ConditionTrue))
			Eventually(refreshFn(c, backupKey)).Should(beCompleted())
		})

		It("should update backup status as failed", func() {
			// get job
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).To(Succeed())

			// update job as completed and failed
			job.Status.Conditions = []batch.JobCondition{
				batch.JobCondition{
					Type:   batch.JobComplete,
					Status: core.ConditionTrue,
				},
				batch.JobCondition{
					Type:   batch.JobFailed,
					Status: core.ConditionTrue,
				},
			}
			Expect(c.Status().Update(context.TODO(), job)).To(Succeed())

			// expect reqoncile request
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			Eventually(refreshFn(c, backupKey)).Should(testutil.BackupHaveCondition(api.BackupComplete, core.ConditionTrue))
			Eventually(refreshFn(c, backupKey)).Should(testutil.BackupHaveCondition(api.BackupFailed, core.ConditionTrue))
			Eventually(refreshFn(c, backupKey)).Should(beCompleted())
		})

	})

	When("a backup is complete", func() {
		BeforeEach(func() {
			// mark backup as completed
			backup.Status.Completed = true
			// create a cluster and a backup
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())
			Expect(c.Create(context.TODO(), backup.Unwrap())).To(Succeed())

			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
			Expect(c.Delete(context.TODO(), backup.Unwrap())).To(Succeed())
		})

		It("should skip creating job", func() {
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).ToNot(Succeed())
		})

		It("should not receive more reconcile requests", func() {
			Consistently(requests, timeout).ShouldNot(Receive(Equal(expectedRequest)))
		})
	})

	When("cluster name is not specified", func() {
		BeforeEach(func() {
			// mark backup as completed
			backup.Spec.ClusterName = ""
			// create a cluster and a backup
			Expect(c.Create(context.TODO(), backup.Unwrap())).To(Succeed())
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

			testutil.DrainChan(requests)
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
			Expect(c.Delete(context.TODO(), backup.Unwrap())).To(Succeed())
		})

		It("should skip creating job", func() {
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).ToNot(Succeed())
		})

		It("should allow updating cluster name", func() {
			// update cluster
			backup.Spec.ClusterName = cluster.Name
			Expect(c.Update(context.TODO(), backup.Unwrap())).To(Succeed())

			// wait for reconcile request
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// check for job to be created
			// NOTE: maybe check in an eventually for job to be created.
			job := &batch.Job{}
			Expect(c.Get(context.TODO(), jobKey, job)).To(Succeed())
		})
	})

	When("backup does not exists", func() {
		BeforeEach(func() {
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
		})

		It("should use backupURI as backupURL", func() {
			backup.Spec.BackupURL = ""
			backup.Spec.BackupURI = "gs://bucket/"
			Expect(c.Create(context.TODO(), backup.Unwrap())).To(Succeed())
			defer c.Delete(context.TODO(), backup.Unwrap())

			// wait for a reconcile request
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			Eventually(refreshFn(c, backupKey)).Should(PointTo(MatchFields(IgnoreExtras, Fields{
				"Spec": MatchFields(IgnoreExtras, Fields{
					"BackupURL": ContainSubstring(backup.Spec.BackupURI),
				}),
			})))
		})
	})
})

func refreshFn(c client.Client, backupKey types.NamespacedName) func() *api.MysqlBackup {
	return func() *api.MysqlBackup {
		backup := &api.MysqlBackup{}
		c.Get(context.TODO(), backupKey, backup)
		return backup
	}
}

func beCompleted() gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Completed": Equal(true),
		}),
	}))
}
