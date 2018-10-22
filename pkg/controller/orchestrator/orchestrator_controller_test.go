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
package orchestrator

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
	fakeOrc "github.com/presslabs/mysql-operator/pkg/orchestrator/fake"
)

var _ = Describe("Orchestrator controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		// orchestrator fake client
		orcClient *fakeOrc.OrcFakeClient
		//timeouts
		noReconcileTime  time.Duration
		reconcileTimeout time.Duration
	)

	BeforeEach(func() {
		orcClient = fakeOrc.New()
		// noReconcileTime + reconcileTimeout > reconcileTimePeriod so that in this time period only one reconcile happens.
		// noReconcileTime represents time required to pass without a reconcile happening (used with Consistently tests)
		// it is set to 90% of the reconcileTimePeriod
		noReconcileTime = reconcileTimePeriod * 9 / 10
		// reconcileTimeout represents time to wait AFTER noReconcileTimeout has passed for a reconciliation to happen
		reconcileTimeout = 3 * (reconcileTimePeriod - noReconcileTime)

		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newReconciler(mgr, orcClient))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = StartTestManager(mgr)

	})

	AfterEach(func() {
		time.Sleep(1 * time.Second)
		close(stop)
	})

	Describe("after creating a new mysql cluster", func() {
		var (
			expectedRequest reconcile.Request
			cluster         *mysqlcluster.MysqlCluster
			secret          *corev1.Secret
			clusterKey      types.NamespacedName
		)

		BeforeEach(func() {
			clusterKey = types.NamespacedName{
				Name:      fmt.Sprintf("cluster-%d", rand.Int31()),
				Namespace: "default",
			}

			expectedRequest = reconcile.Request{
				NamespacedName: clusterKey,
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: clusterKey.Namespace},
				StringData: map[string]string{
					"ROOT_PASSWORD": "this-is-secret",
				},
			}

			theCluster := &api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
				Spec: api.MysqlClusterSpec{
					Replicas:   1,
					SecretName: secret.Name,
				},
			}

			cluster = mysqlcluster.New(theCluster)

			Expect(c.Create(context.TODO(), secret)).To(Succeed())
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

			// update ready nodes
			cluster.Status.ReadyNodes = 1
			Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

			// expect to not receive any event when a cluster is created, but
			// just after reconcile time passed then receive a reconcile event
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))

		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in
			// the test controller plane
			removeAllCreatedResource(c, cluster)
			c.Delete(context.TODO(), secret)
			c.Delete(context.TODO(), cluster.Unwrap())
		})

		It("should trigger reconciliation after noReconcileTime", func() {
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))
		})

		It("should re-register cluster for orchestrator sync when re-starting the controller", func() {
			// restart the controller
			close(stop)
			var recFn reconcile.Reconciler
			mgr, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = mgr.GetClient()
			recFn, requests = SetupTestReconcile(newReconciler(mgr, orcClient))
			Expect(add(mgr, recFn)).To(Succeed())
			stop = StartTestManager(mgr)

			// wait a second for a request
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))
		})

		It("should unregister cluster when deleting it from kubernetes", func() {
			// delete the cluster
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())

			// wait few seconds for a request, in total, noReconcileTime + reconcileTimeout,
			// to catch a reconcile event. This is the request
			// that unregister cluster from orchestrator
			Eventually(requests, noReconcileTime+reconcileTimeout).Should(Receive(Equal(expectedRequest)))

			_, err := orcClient.Cluster(cluster.GetClusterAlias())
			Expect(err).ToNot(Succeed())

			// this is the requests that removes the finalizer and then the
			// cluster is deleted
			Eventually(requests, noReconcileTime+reconcileTimeout).Should(Receive(Equal(expectedRequest)))

			// wait few seconds without request
			Consistently(requests, 3*noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
		})

		It("should be registered in orchestrator", func() {
			// check the cluster is in orchestrator
			insts, err := orcClient.Cluster(cluster.GetClusterAlias())
			Expect(err).To(Succeed())
			Expect(insts).To(haveInstance(cluster.GetPodHostname(0)))
		})
	})
})

func removeAllCreatedResource(c client.Client, cluster *mysqlcluster.MysqlCluster) {
	objs := []runtime.Object{
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
				Namespace: cluster.Namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.HeadlessSVC),
				Namespace: cluster.Namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.MasterService),
				Namespace: cluster.Namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.HealthyNodesService),
				Namespace: cluster.Namespace,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.ConfigMap),
				Namespace: cluster.Namespace,
			},
		},
		&policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.PodDisruptionBudget),
				Namespace: cluster.Namespace,
			},
		},
	}

	for _, obj := range objs {
		c.Delete(context.TODO(), obj)
	}
}

// haveInstance returns a GomegaMatcher that checks if specified host is in
// provided instances list
func haveInstance(host string) gomegatypes.GomegaMatcher {
	return ContainElement(MatchFields(IgnoreExtras, Fields{
		"Key": Equal(orc.InstanceKey{
			Hostname: host,
			Port:     3306,
		}),
	}))
}
