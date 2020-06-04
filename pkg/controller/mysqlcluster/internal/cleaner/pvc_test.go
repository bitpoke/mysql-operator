/*
Copyright 2018 Platform9, Inc

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
	"context"
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var _ = Describe("PVC cleaner", func() {
	var (
		cluster *mysqlcluster.MysqlCluster
		rec     *record.FakeRecorder
		secret  *corev1.Secret
		pvcSpec corev1.PersistentVolumeClaimSpec
	)

	BeforeEach(func() {
		rec = record.NewFakeRecorder(100)
		name := fmt.Sprintf("cluster-%d", rand.Int31())
		ns := "default"

		By("create cluster secret")
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: ns},
			StringData: map[string]string{
				"ROOT_PASSWORD": "this-is-secret",
			},
		}
		Expect(c.Create(context.TODO(), secret)).To(Succeed())

		By("create cluster")
		three := int32(3)
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   &three,
				SecretName: secret.Name,
			},
		})
		Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

		pvcSpec = corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: cluster.GetLabels(),
			},
		}
	})

	AfterEach(func() {
		// remove created cluster
		c.Delete(context.TODO(), cluster.Unwrap())
		c.Delete(context.TODO(), secret)
	})

	Context("with more PVC than replicas", func() {
		var (
			pvcs []corev1.PersistentVolumeClaim
		)
		BeforeEach(func() {

			trueVar := true
			for i := 0; i < 5; i++ {
				pvc := corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("data-%s-mysql-%d", cluster.Name, i),
						Namespace: cluster.Namespace,
						Labels:    cluster.GetSelectorLabels(),
						OwnerReferences: []metav1.OwnerReference{
							metav1.OwnerReference{
								APIVersion: api.SchemeGroupVersion.String(),
								Kind:       "MysqlCluster",
								Name:       cluster.Name,
								UID:        cluster.UID,
								Controller: &trueVar,
							},
						},
					},
					Spec: pvcSpec,
				}
				pvcs = append(pvcs, pvc)
			}

			By("create PVCs")
			for _, pvc := range pvcs {
				Expect(c.Create(context.TODO(), &pvc)).To(Succeed())
			}
		})

		AfterEach(func() {
			for _, pvc := range pvcs {
				c.Delete(context.TODO(), &pvc)
			}
		})

		It("should remove extra PVCs when cluster is scaled down", func() {
			// assert that here are multiple PVCs than needed
			Expect(listClaimsForCluster(c, cluster).Items).To(HaveLen(5))

			// run cleaner
			pvcCleaner := NewPVCCleaner(cluster, options.GetOptions(), rec, c)
			Expect(pvcCleaner.Run(context.TODO())).To(Succeed())

			Expect(listClaimsForCluster(c, cluster).Items).To(HaveLen(3))

			// run cleaner again, should result in no changes
			Expect(pvcCleaner.Run(context.TODO())).To(Succeed())

			Expect(listClaimsForCluster(c, cluster).Items).To(HaveLen(3))
		})

		It("should not remove pvc with 0 index", func() {
			// scale to 0
			zero := int32(0)
			cluster.Spec.Replicas = &zero
			Expect(c.Update(context.TODO(), cluster.Unwrap())).To(Succeed())

			// run cleaner
			pvcCleaner := NewPVCCleaner(cluster, options.GetOptions(), rec, c)
			Expect(pvcCleaner.Run(context.TODO())).To(Succeed())

			pvcs2 := listClaimsForCluster(c, cluster)
			Expect(pvcs2.Items).To(HaveLen(1))
		})
	})
	It("should do nothing when cluster has no claims that belongs to him", func() {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("data-%s-mysql-5", cluster.Name),
				Namespace: cluster.Namespace,
				Labels:    cluster.GetLabels(),
			},
			Spec: pvcSpec,
		}
		Expect(c.Create(context.TODO(), pvc)).To(Succeed())

		// run cleaner
		pvcCleaner := NewPVCCleaner(cluster, options.GetOptions(), rec, c)
		Expect(pvcCleaner.Run(context.TODO())).To(Succeed())

		// check that the pvc is not deleted
		pvcs2 := listClaimsForCluster(c, cluster)
		Expect(pvcs2.Items).To(HaveLen(1))
	})
})

func listClaimsForCluster(c client.Client, cluster *mysqlcluster.MysqlCluster) *corev1.PersistentVolumeClaimList {
	pvcs := &corev1.PersistentVolumeClaimList{}
	opts := &client.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: labels.SelectorFromSet(cluster.GetSelectorLabels()),
	}

	Expect(c.List(context.TODO(), pvcs, opts)).To(Succeed())
	return pvcs
}
