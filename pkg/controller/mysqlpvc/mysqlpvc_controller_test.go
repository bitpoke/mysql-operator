/*
Copyright 2018 Platform9 Inc
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
package mysqlpvc

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("MysqlPvc controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		//timeouts
		noReconcileTime  time.Duration
		reconcileTimeout time.Duration
	)

	BeforeEach(func() {

		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newPvcReconciler(mgr))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = StartTestManager(mgr)

	})

	AfterEach(func() {
		time.Sleep(1 * time.Second)
		close(stop)
	})

	Describe("after creating a new pvc", func() {
		var (
			expectedRequest reconcile.Request
			pvcKey          types.NamespacedName
			pvc             *corev1.PersistentVolumeClaim
		)

		BeforeEach(func() {
			pvcKey = types.NamespacedName{
				Name:      fmt.Sprintf("pvc-%d", rand.Int31()),
				Namespace: "default",
			}

			expectedRequest = reconcile.Request{
				NamespacedName: pvcKey,
			}

			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcKey.Name,
					Namespace: pvcKey.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							Kind: "MysqlCluster",
						},
					},
				},
			}

			Expect(c.Create(context.TODO(), pvc)).To(Succeed())

			// expect to not receive any event when a cluster is created, but
			// just after reconcile time passed then receive a reconcile event
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))

		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in
			// the test controller plane
			removeAllCreatedResource(c, pvc)
			c.Delete(context.TODO(), pvc)
		})

		It("should trigger reconciliation after noReconcileTime", func() {
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))
		})

		It("should re-register pvc for sync when re-starting the controller", func() {
			// restart the controller
			close(stop)
			var recFn reconcile.Reconciler
			mgr, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			c = mgr.GetClient()
			recFn, requests = SetupTestReconcile(newPvcReconciler(mgr))
			Expect(add(mgr, recFn)).To(Succeed())
			stop = StartTestManager(mgr)

			// wait a second for a request
			Consistently(requests, noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
			Eventually(requests, reconcileTimeout).Should(Receive(Equal(expectedRequest)))
		})

		It("should unregister pvc when deleting it from kubernetes", func() {
			// delete the cluster
			Expect(c.Delete(context.TODO(), pvc)).To(Succeed())

			// wait few seconds for a request, in total, noReconcileTime + reconcileTimeout,
			// to catch a reconcile event. This is the request
			// that unregister cluster from orchestrator
			//Eventually(requests, noReconcileTime+reconcileTimeout).Should(Receive(Equal(expectedRequest)))

			//wCluster := wrapcluster.NewMysqlClusterWrapper(cluster)
			//_, err := orcClient.Cluster(wCluster.GetClusterAlias())
			//Expect(err).ToNot(Succeed())

			// this is the requests that removes the finalizer and then the
			// cluster is deleted
			//Eventually(requests, noReconcileTime+reconcileTimeout).Should(Receive(Equal(expectedRequest)))

			// wait few seconds without request
			//Consistently(requests, 3*noReconcileTime).ShouldNot(Receive(Equal(expectedRequest)))
		})
	})
})

func removeAllCreatedResource(c client.Client, pvc *corev1.PersistentVolumeClaim) {
	objs := []runtime.Object{
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
		},
	}

	for _, obj := range objs {
		c.Delete(context.TODO(), obj)
	}
}
