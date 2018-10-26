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
	appsv1 "k8s.io/api/apps/v1"
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

var _ = Describe("Pvc cleaner", func() {
	var (
		cluster     *mysqlcluster.MysqlCluster
		rec         *record.FakeRecorder
		secret      *corev1.Secret
		statefulset *appsv1.StatefulSet
		pvcs        []corev1.PersistentVolumeClaim
		pvcCleaner  *PvcCleaner
	)

	BeforeEach(func() {
		rec = record.NewFakeRecorder(100)
		name := fmt.Sprintf("cluster-%d", rand.Int31())
		ns := "default"

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "the-secret", Namespace: ns},
			StringData: map[string]string{
				"ROOT_PASSWORD": "this-is-secret",
			},
		}
		Expect(c.Create(context.TODO(), secret)).To(Succeed())

		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   3,
				SecretName: secret.Name,
			},
		})

		Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

		pvcSpec := corev1.PersistentVolumeClaimSpec{
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

		for i := 0; i < 5; i++ {
			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("data-%s-mysql-%d", name, i),
					Namespace: ns,
					Labels:    cluster.GetLabels(),
				},
				Spec: pvcSpec,
			}
			pvcs = append(pvcs, pvc)
		}

		for _, pvc := range pvcs {
			Expect(c.Create(context.TODO(), &pvc)).To(Succeed())
		}

		var replicas int32 = 3
		statefulset = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
				Namespace: cluster.Namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: cluster.GetLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: cluster.GetLabels(),
					},
				},
				VolumeClaimTemplates: pvcs,
			},
		}

		Expect(c.Create(context.TODO(), statefulset)).To(Succeed())

		pvcCleaner = NewPvcCleaner(cluster, options.GetOptions())
	})

	It("should remove pvcs when cluster is scaled down", func() {
		Expect(pvcCleaner.Run(context.TODO(), c, nil, rec)).To(Succeed())
		pvcs2 := &corev1.PersistentVolumeClaimList{}
		lo := &client.ListOptions{
			Namespace:     "default",
			LabelSelector: labels.SelectorFromSet(cluster.GetLabels()),
		}
		Expect(c.List(context.TODO(), lo, pvcs2)).To(Succeed())

		Expect(3).To(Equal(len(pvcs2.Items)))
	})
	AfterEach(func() {
		// remove created cluster
		c.Delete(context.TODO(), cluster.Unwrap())
		c.Delete(context.TODO(), secret)
		c.Delete(context.TODO(), statefulset)
		for _, pvc := range pvcs {
			c.Delete(context.TODO(), &pvc)
		}
	})
})
