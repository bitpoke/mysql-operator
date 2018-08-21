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

package mysqlcluster

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

var _ = Describe("MysqlCluster controller", func() {
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
		time.Sleep(1 * time.Second)
		close(stop)
	})

	Describe("when creating a new mysql cluster", func() {
		var (
			expectedRequest reconcile.Request
			cluster         *api.MysqlCluster
			secret          *core.Secret
		)

		BeforeEach(func() {
			name := fmt.Sprintf("cluster-%d", rand.Int31())
			ns := "default"

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			}

			secret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: ns},
				StringData: map[string]string{
					"ROOT_PASSWORD": "this-is-secret",
				},
			}

			cluster = &api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: api.MysqlClusterSpec{
					Replicas:   1,
					SecretName: secret.Name,
				},
			}

			Expect(c.Create(context.TODO(), secret)).To(Succeed())

		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in the test controller plane
			removeAllCreatedResource(c, cluster)
			c.Delete(context.TODO(), secret)
		})

		It("should reconcile the statefulset", func() {
			Expect(c.Create(context.TODO(), cluster)).To(Succeed())
			defer c.Delete(context.TODO(), cluster)
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			sfsKey := types.NamespacedName{
				Name:      cluster.GetNameForResource(api.StatefulSet),
				Namespace: cluster.Namespace,
			}
			statefulSet := &apps.StatefulSet{}
			Eventually(func() error {
				return c.Get(context.TODO(), sfsKey, statefulSet)
			}, timeout).Should(Succeed())

			When("delete the statefulset", func() {
				Expect(c.Delete(context.TODO(), statefulSet)).To(Succeed())
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error {
					return c.Get(context.TODO(), sfsKey, statefulSet)
				}, timeout).Should(Succeed())
			})

		})

		It("should be created all cluster components", func() {
			Expect(c.Create(context.TODO(), cluster)).To(Succeed())
			defer c.Delete(context.TODO(), cluster)
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

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

			testfunc := func() error {
				for _, obj := range objs {
					o := obj.(metav1.Object)
					key := types.NamespacedName{
						Name:      o.GetName(),
						Namespace: o.GetNamespace(),
					}
					err := c.Get(context.TODO(), key, obj)
					if err != nil {
						return err
					}
				}
				return nil
			}

			Eventually(testfunc, timeout).Should(Succeed())

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
		Expect(c.Delete(context.TODO(), obj)).To(Succeed())
	}
}
