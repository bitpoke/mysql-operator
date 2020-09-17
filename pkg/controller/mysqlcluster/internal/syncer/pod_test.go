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
	"context"
	"fmt"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	two = int32(2)
)

var _ = Describe("Pod syncer", func() {
	var (
		cluster *mysqlcluster.MysqlCluster
	)

	BeforeEach(func() {
		name := fmt.Sprintf("cluster-%d", rand.Int31())
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Status: api.MysqlClusterStatus{
				ReadyNodes: 2,
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   &two,
				SecretName: "the-secret",
			},
		})

		Expect(c.Create(context.TODO(), cluster.Unwrap())).To(Succeed())

		// create pods and update cluster nodes conditions
		cluster.UpdateNodeConditionStatus(cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionTrue)
		Expect(c.Create(context.TODO(), getPod(cluster, 0))).To(Succeed())
		cluster.UpdateNodeConditionStatus(cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionFalse)
		Expect(c.Create(context.TODO(), getPod(cluster, 1))).To(Succeed())

		Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

		// run the syncers
		_, err := NewPodSyncer(c, scheme.Scheme, cluster, cluster.GetPodHostname(0)).Sync(context.TODO())
		Expect(err).To(Succeed())
		_, err = NewPodSyncer(c, scheme.Scheme, cluster, cluster.GetPodHostname(1)).Sync(context.TODO())
		Expect(err).To(Succeed())

	})

	AfterEach(func() {
		// remove created cluster
		c.Delete(context.TODO(), cluster.Unwrap())
		// remove all created pods
		podList := &core.PodList{}
		c.List(context.TODO(), podList, &client.ListOptions{})
		for _, pod := range podList.Items {
			c.Delete(context.TODO(), &pod)
		}
	})

	It("should label the pods acordingly", func() {
		pod0 := &core.Pod{}
		Expect(c.Get(context.TODO(), getPodKey(cluster, 0), pod0)).To(Succeed())
		Expect(pod0.ObjectMeta.Labels).To(ContainElement(Equal("master")))
		Expect(pod0.ObjectMeta.Labels).To(ContainElement(Equal("yes")))

		pod1 := &core.Pod{}
		Expect(c.Get(context.TODO(), getPodKey(cluster, 1), pod1)).To(Succeed())
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("replica")))
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("no")))

	})

	It("should fail if pod does not exist", func() {
		_, err := NewPodSyncer(c, scheme.Scheme, cluster, cluster.GetPodHostname(2)).Sync(context.TODO())
		Expect(err).ToNot(Succeed())
	})

	It("should mark pods as healthy", func() {
		cluster.UpdateNodeConditionStatus(cluster.GetPodHostname(1), api.NodeConditionLagged, core.ConditionFalse)
		cluster.UpdateNodeConditionStatus(cluster.GetPodHostname(1), api.NodeConditionReplicating, core.ConditionTrue)
		// update cluster
		Expect(c.Status().Update(context.TODO(), cluster.Unwrap())).To(Succeed())

		// call the syncer
		_, err := NewPodSyncer(c, scheme.Scheme, cluster, cluster.GetPodHostname(1)).Sync(context.TODO())
		Expect(err).To(Succeed())

		pod1 := &core.Pod{}
		Expect(c.Get(context.TODO(), getPodKey(cluster, 1), pod1)).To(Succeed())
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("replica")))
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("yes")))
	})
})

func getPodName(cluster *mysqlcluster.MysqlCluster, id int) string {
	return fmt.Sprintf("%s-%d", cluster.GetNameForResource(mysqlcluster.StatefulSet), id)
}

func getPodKey(cluster *mysqlcluster.MysqlCluster, id int) types.NamespacedName {
	return types.NamespacedName{
		Name:      getPodName(cluster, id),
		Namespace: cluster.Namespace,
	}
}

func getPod(cluster *mysqlcluster.MysqlCluster, id int) *core.Pod {
	return &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPodName(cluster, id),
			Namespace: cluster.Namespace,
		},
		Spec: core.PodSpec{
			Containers: []core.Container{
				core.Container{
					Name:  "first",
					Image: "image:latest",
				},
			},
		},
	}
}
