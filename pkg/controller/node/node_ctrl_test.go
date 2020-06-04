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

// nolint: errcheck
package node

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

var (
	one = int32(1)
)

var _ = Describe("MysqlNode controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client

		sqli *fakeSQLRunner
	)

	BeforeEach(func() {
		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		sqli = &fakeSQLRunner{}
		newNodeConn := func(dsn, host string) SQLInterface {
			return sqli
		}

		recFn, requests = testutil.SetupTestReconcile(newReconciler(mgr, newNodeConn))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = testutil.StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("when creating a new cluster with new nodes", func() {
		var (
			expectedRequest func(int) reconcile.Request
			cluster         *mysqlcluster.MysqlCluster
			secret          *corev1.Secret
		)

		BeforeEach(func() {
			cluster = mysqlcluster.New(&api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("cluster-%d", rand.Int31()),
					Namespace: "default",
					Annotations: map[string]string{
						"mysql.presslabs.org/version": "300",
					},
				},
				Spec: api.MysqlClusterSpec{
					Replicas:   &one,
					SecretName: "the-secret",
				},
			})

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cluster.GetNameForResource(mysqlcluster.Secret),
					Namespace: cluster.Namespace,
				},
				StringData: map[string]string{
					"OPERATOR_USER":        "u",
					"OPERATOR_PASSWORD":    "up",
					"REPLICATION_USER":     "ru",
					"REPLICATION_PASSWORD": "rup",
				},
			}

			expectedRequest = func(i int) reconcile.Request {
				return reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), i),
						Namespace: cluster.Namespace,
					},
				}
			}

			By("create the MySQL cluster")
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())
			// ensure that cluster is created, else test may be flaky
			Eventually(testutil.RefreshFn(c, cluster.Unwrap())).ShouldNot(BeNil())

			By("create MySQL pod")
			pod := getOrCreatePod(c, cluster, 0)
			setPodPhase(c, pod, corev1.PodRunning)
			// no event for create
			// pod update as running
			Eventually(requests).Should(Receive(Equal(expectedRequest(0))))
			// pod update for pod conditions (NodeInitializedConditionType)
			Eventually(requests).Should(Receive(Equal(expectedRequest(0))))
		})

		AfterEach(func() {
			// NOTE: at this moment the reconcile func is running and modifying a resource will trigger
			// a reconcile event which will be logged and that can be confusing when debugging.
			By("cleanup created resources")

			// manually delete all created resources because GC isn't enabled in the test controller plane
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
			podList := &corev1.PodList{}
			Expect(c.List(context.TODO(), podList)).To(Succeed())
			for _, pod := range podList.Items {
				Expect(c.Delete(context.TODO(), &pod)).To(Succeed())
			}
		})

		It("should receive pod update event", func() {
			pod1 := getOrCreatePod(c, cluster, 0)
			setPodPhase(c, pod1, corev1.PodRunning)
			updatePodStatusCondition(pod1, mysqlcluster.NodeInitializedConditionType, corev1.ConditionFalse, "", "")
			Expect(c.Status().Update(context.TODO(), pod1)).To(Succeed())
			Eventually(requests).Should(Receive(Equal(expectedRequest(0))))

		})

		It("should not receive pod update when initialized", func() {
			pod1 := getOrCreatePod(c, cluster, 0)

			// when pod is ready should not be triggered a reconcile event
			By("update pod status to Pending")
			setPodPhase(c, pod1, corev1.PodPending)
			Consistently(requests).ShouldNot(Receive(Equal(expectedRequest(0))))

			// mark pod running, should init node
			By("update pod status to Running")
			setPodPhase(c, pod1, corev1.PodRunning)
			Eventually(requests).Should(Receive(Equal(expectedRequest(0))))

			// mark pod ready, should not init node anymore
			updatePodStatusCondition(pod1, corev1.PodReady, corev1.ConditionTrue, "", "")
			Expect(c.Status().Update(context.TODO(), pod1)).To(Succeed())
			Consistently(requests).ShouldNot(Receive(Equal(expectedRequest(0))))
		})

		It("should have mysql initialized set when initialization succeed", func() {
			pod1 := getOrCreatePod(c, cluster, 0)
			setPodPhase(c, pod1, corev1.PodRunning)
			Expect(pod1).To(testutil.PodHaveCondition(mysqlcluster.NodeInitializedConditionType, corev1.ConditionTrue))

		})

	})
})

func podKey(cluster *mysqlcluster.MysqlCluster, index int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), index),
		Namespace: cluster.Namespace,
	}
}

// nolint: unparam
func getOrCreatePod(c client.Client, cluster *mysqlcluster.MysqlCluster, index int) *corev1.Pod {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), podKey(cluster, index), pod)
	if errors.IsNotFound(err) {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), index),
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "mysql.presslabs.org",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "dummy",
						Image: "dummy",
					},
				},
			},
		}
		Expect(c.Create(context.TODO(), pod)).To(Succeed())
		return pod
	}

	Expect(err).To(BeNil())
	return pod
}

func setPodPhase(c client.StatusClient, pod *corev1.Pod, phase corev1.PodPhase) {
	pod.Status.Phase = phase
	Expect(c.Status().Update(context.TODO(), pod)).To(Succeed())
}
