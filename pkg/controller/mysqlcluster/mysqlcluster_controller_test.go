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
package mysqlcluster

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

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

type clusterComponents []runtime.Object

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
		close(stop)
	})

	Describe("when creating a new mysql cluster", func() {
		var (
			expectedRequest reconcile.Request
			cluster         *api.MysqlCluster
			secret          *core.Secret
			components      clusterComponents
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
					Replicas:   2,
					SecretName: secret.Name,
				},
			}

			components = []runtime.Object{
				&apps.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&core.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql-nodes", name),
						Namespace: cluster.Namespace,
					},
				},
				&core.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql-master", name),
						Namespace: cluster.Namespace,
					},
				},
				&core.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&core.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&policy.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
			}

			Expect(c.Create(context.TODO(), secret)).To(Succeed())
			Expect(c.Create(context.TODO(), cluster)).To(Succeed())

			// Initial reconciliation
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			// Reconcile triggered by components being created and status being
			// updated
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// We need to make sure that the controller does not create infinite
			// loops
			Consistently(requests, time.Second).ShouldNot(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in the test controller plane
			removeAllCreatedResource(c, components)
			c.Delete(context.TODO(), secret)
			c.Delete(context.TODO(), cluster)
		})

		It("should create a cluster", func() {

			newCluster := &api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "new-cluster", Namespace: "default"},
				Spec: api.MysqlClusterSpec{
					Replicas:   1,
					SecretName: secret.Name,
				},
			}
			defer c.Delete(context.TODO(), newCluster)
			Expect(c.Create(context.TODO(), newCluster)).To(Succeed())

			newER := reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "new-cluster", Namespace: "default"},
			}

			Eventually(requests, timeout).Should(Receive(Equal(newER)))
			Eventually(requests, timeout).Should(Receive(Equal(newER)))
		})

		It("should reconcile the statefulset", func() {
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

		It("should create all the components when cluster is created", func() {
			for _, obj := range components {
				o := obj.(metav1.Object)
				key := types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()}

				Eventually(func() error { return c.Get(context.TODO(), key, obj) }).Should(Succeed())
				When("delete component", func() {
					Expect(c.Delete(context.TODO(), obj)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error {
						return c.Get(context.TODO(), key, obj)
					}, timeout).Should(Succeed())
				})

			}
		})

		It("should have revision annotation on statefulset", func() {
			// wait for the second update, else a race condition can happened
			// with secret resource version.
			//Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			sfsKey := types.NamespacedName{
				Name:      cluster.GetNameForResource(api.StatefulSet),
				Namespace: cluster.Namespace,
			}
			statefulSet := &apps.StatefulSet{}
			Eventually(func() error {
				return c.Get(context.TODO(), sfsKey, statefulSet)
			}, timeout).Should(Succeed())

			cfgMap := &core.ConfigMap{}
			Expect(c.Get(context.TODO(), types.NamespacedName{
				Name:      cluster.GetNameForResource(api.ConfigMap),
				Namespace: cluster.Namespace,
			}, cfgMap)).To(Succeed())
			Expect(c.Get(context.TODO(), types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			}, secret)).To(Succeed())

			Expect(statefulSet.Spec.Template.ObjectMeta.Annotations["config_rev"]).To(Equal(cfgMap.ResourceVersion))
			Expect(statefulSet.Spec.Template.ObjectMeta.Annotations["secret_rev"]).To(Equal(secret.ResourceVersion))
		})

		It("should have set ready condition", func() {
			// get statefulset
			sfsKey := types.NamespacedName{
				Name:      cluster.GetNameForResource(api.StatefulSet),
				Namespace: cluster.Namespace,
			}
			statefulSet := &apps.StatefulSet{}
			Eventually(func() error {
				return c.Get(context.TODO(), sfsKey, statefulSet)
			}, timeout).Should(Succeed())

			// update statefulset condition
			statefulSet.Status.ReadyReplicas = 2
			statefulSet.Status.Replicas = 2
			Expect(c.Status().Update(context.TODO(), statefulSet)).To(Succeed())

			// expect a reconcile event
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(getClusterConditions(c, cluster), timeout).Should(haveCondWithStatus(api.ClusterConditionReady, core.ConditionTrue))
		})
	})

})

func removeAllCreatedResource(c client.Client, clusterComps []runtime.Object) {
	for _, obj := range clusterComps {
		c.Delete(context.TODO(), obj)
	}
}

// getClusterConditions is a helper func that returns a functions that returns cluster status conditions
func getClusterConditions(c client.Client, cluster *api.MysqlCluster) func() []api.ClusterCondition {
	return func() []api.ClusterCondition {
		cl := &api.MysqlCluster{}
		c.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cl)
		return cl.Status.Conditions
	}
}

// haveCondWithStatus is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
func haveCondWithStatus(condType api.ClusterConditionType, status core.ConditionStatus) gomegatypes.GomegaMatcher {
	return ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	}))
}
