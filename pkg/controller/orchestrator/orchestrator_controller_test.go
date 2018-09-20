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

	"golang.org/x/net/context"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const timeout = time.Second * 2

var _ = Describe("Orchestrator controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		// time between reconciliations
		reconcileTime time.Duration
	)

	BeforeEach(func() {
		reconcileTime = time.Second - 100*time.Millisecond

		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newReconciler(mgr))
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
			cluster         *api.MysqlCluster
			secret          *core.Secret
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

			secret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: clusterKey.Namespace},
				StringData: map[string]string{
					"ROOT_PASSWORD": "this-is-secret",
				},
			}

			cluster = &api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
				Spec: api.MysqlClusterSpec{
					Replicas:   1,
					SecretName: secret.Name,
				},
			}

			Expect(c.Create(context.TODO(), secret)).To(Succeed())
			Expect(c.Create(context.TODO(), cluster)).To(Succeed())

			// expect to not receive any event when a cluster is created, but
			// just after reconcile time passed then receive a reconcile event
			Consistently(requests, reconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			// Initial reconciliation
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			// Reconcile triggered by components being created and status being
			// updated
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// We need to make sure that the controller does not create infinite
			// loops
			Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in
			// the test controller plane
			removeAllCreatedResource(c, cluster)
			c.Delete(context.TODO(), secret)
		})

		It("should re-register cluster for orchestrator sync when re-starting the controller", func() {
			// restart the controller
			close(stop)
			var recFn reconcile.Reconciler
			mgr, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = mgr.GetClient()
			recFn, requests = SetupTestReconcile(newReconciler(mgr))
			Expect(add(mgr, recFn)).To(Succeed())
			stop = StartTestManager(mgr)

			// wait a second for a request
			Consistently(requests, reconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests).Should(Receive(Equal(expectedRequest)))
		})

		It("should unregister cluster when deleting it from kubernetes", func() {
			// delete the cluster
			Expect(c.Delete(context.TODO(), cluster)).To(Succeed())

			// wait two seconds without request
			Consistently(requests, 2*reconcileTime).ShouldNot(Receive(Equal(expectedRequest)))

		})
	})
})

func removeAllCreatedResource(c client.Client, cluster *api.MysqlCluster) {
	objs := []runtime.Object{
		&apps.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.StatefulSet),
				Namespace: cluster.Namespace,
			},
		},
		&core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.HeadlessSVC),
				Namespace: cluster.Namespace,
			},
		},
		&core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.MasterService),
				Namespace: cluster.Namespace,
			},
		},
		&core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.HealthyNodesService),
				Namespace: cluster.Namespace,
			},
		},
		&core.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.ConfigMap),
				Namespace: cluster.Namespace,
			},
		},
		&policy.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(api.PodDisruptionBudget),
				Namespace: cluster.Namespace,
			},
		},
	}

	for _, obj := range objs {
		c.Delete(context.TODO(), obj)
	}
}
