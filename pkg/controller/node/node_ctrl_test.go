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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

const timeout = time.Second * 2

var (
	one = int32(1)
	two = int32(2)
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

			By("create the MySQL node")
			getOrCreatePod(c, cluster, 0)

		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in the test controller plane
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
			podList := &corev1.PodList{}
			c.List(context.TODO(), nil, podList)
			for _, pod := range podList.Items {
				c.Delete(context.TODO(), &pod)
			}

		})

		It("should receive pod update event", func() {
			pod1 := getOrCreatePod(c, cluster, 0)
			updatePodStatusCondition(pod1, corev1.PodReady, corev1.ConditionFalse, "", "")
			Expect(c.Update(context.TODO(), pod1)).To(Succeed())
			Eventually(requests).Should(Receive(Equal(expectedRequest(0))))

		})
	})
})

func objToKey(o runtime.Object) types.NamespacedName {
	obj, _ := o.(*corev1.Pod)
	return types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}
}

func podKey(cluster *mysqlcluster.MysqlCluster, index int) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), index),
		Namespace: cluster.Namespace,
	}
}

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

func haveLabelWithValue(label, value string) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Labels": HaveKeyWithValue(label, value),
		}),
	}))
}
