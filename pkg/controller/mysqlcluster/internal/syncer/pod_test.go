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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	clusterwrap "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
)

var _ = Describe("Pod syncer", func() {
	var (
		cluster  *api.MysqlCluster
		wCluster *clusterwrap.MysqlCluster
		rec      *record.FakeRecorder
	)

	BeforeEach(func() {
		name := fmt.Sprintf("cluster-%d", rand.Int31())
		rec = record.NewFakeRecorder(100)
		cluster = &api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Status: api.MysqlClusterStatus{
				ReadyNodes: 2,
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   2,
				SecretName: "the-secret",
			},
		}
		wCluster = clusterwrap.NewMysqlClusterWrapper(cluster)
		Expect(c.Create(context.TODO(), cluster)).To(Succeed())

		// create pods and update cluster nodes conditions
		wCluster.UpdateNodeConditionStatus(wCluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionTrue)
		Expect(c.Create(context.TODO(), getPod(cluster, 0))).To(Succeed())
		wCluster.UpdateNodeConditionStatus(wCluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionFalse)
		Expect(c.Create(context.TODO(), getPod(cluster, 1))).To(Succeed())

		Expect(c.Status().Update(context.TODO(), cluster)).To(Succeed())

		// run the syncers
		syncer0 := NewPodSyncer(cluster, wCluster.GetPodHostname(0))
		Expect(syncer.Sync(context.TODO(), syncer0, c, nil, rec)).To(Succeed())
		syncer1 := NewPodSyncer(cluster, wCluster.GetPodHostname(1))
		Expect(syncer.Sync(context.TODO(), syncer1, c, nil, rec)).To(Succeed())

	})

	AfterEach(func() {
		// remove created cluster
		c.Delete(context.TODO(), cluster)
		// remove all created pods
		podList := &core.PodList{}
		c.List(context.TODO(), &client.ListOptions{}, podList)
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
		syncer2 := NewPodSyncer(cluster, wCluster.GetPodHostname(2))
		Expect(syncer.Sync(context.TODO(), syncer2, c, nil, rec)).ToNot(Succeed())
	})

	It("should mark pods as healty", func() {
		wCluster.UpdateNodeConditionStatus(wCluster.GetPodHostname(1), api.NodeConditionLagged, core.ConditionFalse)
		wCluster.UpdateNodeConditionStatus(wCluster.GetPodHostname(1), api.NodeConditionReplicating, core.ConditionTrue)
		// update cluster
		Expect(c.Status().Update(context.TODO(), cluster)).To(Succeed())

		// call the syncer
		syncer1 := NewPodSyncer(cluster, wCluster.GetPodHostname(1))
		Expect(syncer.Sync(context.TODO(), syncer1, c, nil, rec)).To(Succeed())

		pod1 := &core.Pod{}
		Expect(c.Get(context.TODO(), getPodKey(cluster, 1), pod1)).To(Succeed())
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("replica")))
		Expect(pod1.ObjectMeta.Labels).To(ContainElement(Equal("yes")))
	})
})

func getPodName(cluster *api.MysqlCluster, id int) string {
	return fmt.Sprintf("%s-%d", cluster.GetNameForResource(api.StatefulSet), id)
}

func getPodKey(cluster *api.MysqlCluster, id int) types.NamespacedName {
	return types.NamespacedName{
		Name:      getPodName(cluster, id),
		Namespace: cluster.Namespace,
	}
}

func getPod(cluster *api.MysqlCluster, id int) *core.Pod {
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
