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

// nolint: errcheck, unparam
package mysqlcluster

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

type clusterComponents []runtime.Object

const timeout = time.Second * 2

var (
	two = int32(2)
)

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

		recFn, requests = testutil.SetupTestReconcile(newReconciler(mgr))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = testutil.StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("when creating a new mysql cluster", func() {
		var (
			expectedRequest reconcile.Request
			cluster         *mysqlcluster.MysqlCluster
			clusterKey      types.NamespacedName
			secret          *corev1.Secret
			components      clusterComponents
		)

		BeforeEach(func() {
			name := fmt.Sprintf("cluster-%d", rand.Int31())
			ns := "default"

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: ns},
				StringData: map[string]string{
					"ROOT_PASSWORD": "this-is-secret",
				},
			}

			cluster = mysqlcluster.New(&api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
					Annotations: map[string]string{
						"mysql.presslabs.org/version": "300",
					},
				},
				Spec: api.MysqlClusterSpec{
					Replicas:   &two,
					SecretName: secret.Name,
					VolumeSpec: api.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{},
					},
				},
			})
			clusterKey = types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}

			components = []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql-operated", name),
						Namespace: cluster.Namespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql-master", name),
						Namespace: cluster.Namespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
				&policyv1beta1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-mysql", name),
						Namespace: cluster.Namespace,
					},
				},
			}

			By("create the MySQL cluster")
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

			// Initial reconciliation
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			// Reconcile triggered by components being created and status being
			// updated
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			// some extra reconcile requests may appear
			testutil.DrainChan(requests)

		})

		AfterEach(func() {
			// manually delete all created resources because GC isn't enabled in the test controller plane
			removeAllCreatedResource(c, components)
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
		})

		It("should have only one reconcile request", func() {
			// We need to make sure that the controller does not create infinite
			// loops
			Consistently(requests, 5*time.Second).ShouldNot(Receive(Equal(expectedRequest)))
		})

		DescribeTable("the reconciler",
			func(nameFmt string, obj runtime.Object) {
				key := types.NamespacedName{
					Name:      fmt.Sprintf(nameFmt, cluster.Name),
					Namespace: cluster.Namespace,
				}

				By("creating the resource when the cluster is created")
				Eventually(func() error { return c.Get(context.TODO(), key, obj) }, timeout).Should(Succeed())

				By("reacreating the resource when it gets deleted")
				// Delete the resource and expect Reconcile to be called
				Expect(c.Delete(context.TODO(), obj)).To(Succeed())
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(func() error { return c.Get(context.TODO(), key, obj) }, timeout).Should(Succeed())
			},
			Entry("reconciles the statefulset", "%s-mysql", &appsv1.StatefulSet{}),
			Entry("reconciles the master service", "%s-mysql-master", &corev1.Service{}),
			Entry("reconciles the operator secret", "%s-mysql-operated", &corev1.Secret{}),
			Entry("reconciles the config map", "%s-mysql", &corev1.ConfigMap{}),
			Entry("reconciles the pod disruption budget", "%s-mysql", &policyv1beta1.PodDisruptionBudget{}),
		)

		Describe("the reconciler", func() {
			It("should update secret and configmap revision annotations on statefulset", func() {
				sfsKey := types.NamespacedName{
					Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
					Namespace: cluster.Namespace,
				}
				statefulSet := &appsv1.StatefulSet{}
				Eventually(func() error {
					return c.Get(context.TODO(), sfsKey, statefulSet)
				}, timeout).Should(Succeed())

				cfgMap := &corev1.ConfigMap{}
				Expect(c.Get(context.TODO(), types.NamespacedName{
					Name:      cluster.GetNameForResource(mysqlcluster.ConfigMap),
					Namespace: cluster.Namespace,
				}, cfgMap)).To(Succeed())
				sct := &corev1.Secret{}
				Expect(c.Get(context.TODO(), types.NamespacedName{
					Name:      cluster.GetNameForResource(mysqlcluster.Secret),
					Namespace: secret.Namespace,
				}, sct)).To(Succeed())

				Expect(statefulSet.Spec.Template.ObjectMeta.Annotations["config_rev"]).To(Equal(cfgMap.ResourceVersion))
				Expect(statefulSet.Spec.Template.ObjectMeta.Annotations["secret_rev"]).To(Equal(sct.ResourceVersion))
			})
			It("should update cluster ready nodes", func() {
				// get statefulset
				sfsKey := types.NamespacedName{
					Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
					Namespace: cluster.Namespace,
				}
				statefulSet := &appsv1.StatefulSet{}
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

				// check ready nodes are updated
				Eventually(testutil.RefreshFn(c, cluster.Unwrap())).Should(testutil.HaveClusterStatusReadyNodes(2))
			})
			It("should label pods as healthy and as master accordingly", func() {
				pod0 := getPod(cluster, 0)
				Expect(c.Create(context.TODO(), pod0)).To(Succeed())
				pod1 := getPod(cluster, 1)
				Expect(c.Create(context.TODO(), pod1)).To(Succeed())
				pod2 := getPod(cluster, 2)
				Expect(c.Create(context.TODO(), pod2)).To(Succeed())

				// update cluster conditions
				By("update cluster status")
				Expect(c.Get(context.TODO(), clusterKey, cluster.Unwrap())).To(Succeed())
				cluster.Status.Nodes = []api.NodeStatus{
					nodeStatusForPod(cluster, pod0, true, false, false),
					nodeStatusForPod(cluster, pod1, false, false, true),
					nodeStatusForPod(cluster, pod2, false, false, false),
				}
				Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// assert pods labels
				// master
				Expect(c.Get(context.TODO(), objToKey(pod0), pod0)).To(Succeed())
				Expect(pod0).To(haveLabelWithValue("role", "master"))
				Expect(pod0).To(haveLabelWithValue("healthy", "yes"))

				// replica
				Expect(c.Get(context.TODO(), objToKey(pod1), pod1)).To(Succeed())
				Expect(pod1).To(haveLabelWithValue("role", "replica"))
				Expect(pod1).To(haveLabelWithValue("healthy", "yes"))
			})
			It("should label pods as master even if other pods does not exists", func() {
				pod0 := getPod(cluster, 0)
				Expect(c.Create(context.TODO(), pod0)).To(Succeed())
				pod1 := getPod(cluster, 1)
				pod2 := getPod(cluster, 2)

				// update cluster conditions
				By("update cluster status")
				Expect(c.Get(context.TODO(), clusterKey, cluster.Unwrap())).To(Succeed())
				cluster.Status.Nodes = []api.NodeStatus{
					nodeStatusForPod(cluster, pod0, true, false, false),
					nodeStatusForPod(cluster, pod1, false, false, false),
					nodeStatusForPod(cluster, pod2, false, false, false),
				}
				Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// assert pods labels
				// master
				// TODO: this may be a flaky test so use Eventually
				Expect(c.Get(context.TODO(), objToKey(pod0), pod0)).To(Succeed())
				Expect(pod0).To(haveLabelWithValue("role", "master"))
				Expect(pod0).To(haveLabelWithValue("healthy", "yes"))

				// check pod is not created
				Expect(c.Get(context.TODO(), objToKey(pod1), pod1)).ToNot(Succeed())
			})
			It("should label as unhealthy if lagged", func() {
				pod0 := getPod(cluster, 0)
				Expect(c.Create(context.TODO(), pod0)).To(Succeed())
				pod1 := getPod(cluster, 1)
				Expect(c.Create(context.TODO(), pod1)).To(Succeed())

				// update cluster conditions
				By("update cluster status")
				Expect(c.Get(context.TODO(), clusterKey, cluster.Unwrap())).To(Succeed())
				cluster.Status.Nodes = []api.NodeStatus{
					nodeStatusForPod(cluster, pod0, false, true, false),
					nodeStatusForPod(cluster, pod1, true, false, true),
				}
				Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// assert pods labels
				// master
				Expect(c.Get(context.TODO(), objToKey(pod0), pod0)).To(Succeed())
				Expect(pod0).To(haveLabelWithValue("role", "replica"))
				Expect(pod0).To(haveLabelWithValue("healthy", "no"))

				// replica
				Expect(c.Get(context.TODO(), objToKey(pod1), pod1)).To(Succeed())
				Expect(pod1).To(haveLabelWithValue("role", "master"))
				Expect(pod1).To(haveLabelWithValue("healthy", "yes"))
			})
			It("should update cluster secret with keys", func() {
				s := &corev1.Secret{}
				sKey := types.NamespacedName{
					Name:      cluster.GetNameForResource(mysqlcluster.Secret),
					Namespace: secret.Namespace,
				}
				Expect(c.Get(context.TODO(), sKey, s)).To(Succeed())

				Expect(s.OwnerReferences).To(HaveLen(1))

				// check for keys to be set
				Expect(s.Data).To(HaveKey("REPLICATION_USER"))
				Expect(s.Data).To(HaveKey("REPLICATION_PASSWORD"))
				Expect(s.Data).To(HaveKey("METRICS_EXPORTER_USER"))
				Expect(s.Data).To(HaveKey("METRICS_EXPORTER_PASSWORD"))
				Expect(s.Data).To(HaveKey("ORC_TOPOLOGY_USER"))
				Expect(s.Data).To(HaveKey("ORC_TOPOLOGY_PASSWORD"))
			})

			It("should not set owner reference on cluster secret", func() {
				s := &corev1.Secret{}
				sKey := types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				}
				Expect(c.Get(context.TODO(), sKey, s)).To(Succeed())

				Expect(s.OwnerReferences).To(HaveLen(0), "should have no owner reference set")
			})
		})
	})

	Context("with secret and uninitialized cluster", func() {
		var (
			expectedRequest reconcile.Request
			cluster         *mysqlcluster.MysqlCluster
			secret          *corev1.Secret
		)

		BeforeEach(func() {
			name := fmt.Sprintf("cluster-%d", rand.Int31())
			ns := "default"

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: ns},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: ns},
				StringData: map[string]string{
					"ROOT_PASSWORD": "this-is-secret",
				},
			}

			cluster = mysqlcluster.New(&api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
					Annotations: map[string]string{
						"mysql.presslabs.org/version": "300",
					},
				},
				Spec: api.MysqlClusterSpec{
					Replicas:   &two,
					SecretName: secret.Name,
				},
			})

			Expect(c.Create(context.TODO(), secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
			Expect(c.Delete(context.TODO(), cluster.Unwrap())).To(Succeed())
		})

		It("should set emptyDir as data volume", func() {
			cluster.Spec.VolumeSpec = api.VolumeSpec{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}

			// crete cluster
			Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			sts := &appsv1.StatefulSet{}
			stsKey := types.NamespacedName{
				Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
				Namespace: cluster.Namespace,
			}

			Expect(c.Get(context.TODO(), stsKey, sts)).To(Succeed())

			Expect(sts.Spec.Template.Spec.Volumes).To(ContainElement(Equal(
				corev1.Volume{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: cluster.Spec.VolumeSpec.EmptyDir,
					},
				},
			)))
		})

	})
})

func removeAllCreatedResource(c client.Client, clusterComps []runtime.Object) {
	for _, obj := range clusterComps {
		c.Delete(context.TODO(), obj)
	}
}

func objToKey(o runtime.Object) types.NamespacedName {
	obj, _ := o.(*corev1.Pod)
	return types.NamespacedName{
		Name:      obj.Name,
		Namespace: obj.Namespace,
	}
}

func getPod(cluster *mysqlcluster.MysqlCluster, index int) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), index),
			Namespace: cluster.Namespace,
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
}

func nodeStatusForPod(cluster *mysqlcluster.MysqlCluster, pod *corev1.Pod, master, lagged, replicating bool) api.NodeStatus {
	name := fmt.Sprintf("%s.%s.%s", pod.Name, cluster.GetNameForResource(mysqlcluster.HeadlessSVC), pod.Namespace)

	boolToStatus := func(c bool) corev1.ConditionStatus {
		if c {
			return corev1.ConditionTrue
		}
		return corev1.ConditionFalse
	}

	t := time.Now()

	return api.NodeStatus{
		Name: name,
		Conditions: []api.NodeCondition{
			{
				Type:               api.NodeConditionMaster,
				Status:             boolToStatus(master),
				LastTransitionTime: metav1.NewTime(t),
			},
			{
				Type:               api.NodeConditionLagged,
				Status:             boolToStatus(lagged),
				LastTransitionTime: metav1.NewTime(t),
			},
			{
				Type:               api.NodeConditionReplicating,
				Status:             boolToStatus(replicating),
				LastTransitionTime: metav1.NewTime(t),
			},
		},
	}
}

func haveLabelWithValue(label, value string) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Labels": HaveKeyWithValue(label, value),
		}),
	}))
}
