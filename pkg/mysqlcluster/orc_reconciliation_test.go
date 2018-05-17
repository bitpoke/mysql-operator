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
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	"github.com/presslabs/mysql-operator/pkg/util/options"
	fakeOrc "github.com/presslabs/mysql-operator/pkg/util/orchestrator/fake"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

func TestReconciliation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test reconciliation")
}

var _ = Describe("Mysql cluster reconcilation", func() {

	var (
		client    *fake.Clientset
		myClient  *fakeMyClient.Clientset
		rec       *record.FakeRecorder
		cluster   *api.MysqlCluster
		factory   *cFactory
		ctx       context.Context
		orcClient *fakeOrc.FakeOrc
		namespace = tutil.Namespace
	)

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		myClient = fakeMyClient.NewSimpleClientset()
		rec = record.NewFakeRecorder(100)
		ctx = context.TODO()
		orcClient = fakeOrc.New()
		cluster = tutil.NewFakeCluster("asd")
		factory = &cFactory{
			cluster:    cluster,
			opt:        options.GetOptions(),
			client:     client,
			myClient:   myClient,
			namespace:  namespace,
			rec:        rec,
			configHash: "1",
			secretHash: "1",
			orcClient:  orcClient,
		}
	})

	Describe("Update status from orc", func() {
		Context("cluster does not exists in orc", func() {
			It("should register intro orc", func() {
				cluster.Status.ReadyNodes = 1
				Ω(factory.ReconcileORC(ctx)).Should(Succeed())
				Expect(orcClient.CheckDiscovered("asd-mysql-0.asd-mysql.default")).To(Equal(true))
			})

			It("should update status", func() {
				orcClient.AddInstance("asd.default", "asd-mysql-0.asd-mysql.default",
					true, -1, false)
				orcClient.AddRecoveries("asd.default", 1, true)
				Ω(factory.ReconcileORC(ctx)).Should(Succeed())
				Expect(cluster.Status.Nodes[0].GetCondition(api.NodeConditionMaster).Status).To(
					Equal(core.ConditionTrue))

				Expect(getCCond(
					cluster.Status.Conditions, api.ClusterConditionFailoverAck).Status).To(
					Equal(core.ConditionFalse))
			})

			It("should have pending recoveries", func() {
				orcClient.AddInstance("asd.default", "asd-mysql-0.asd-mysql.default",
					true, -1, false)
				orcClient.AddRecoveries("asd.default", 11, false)
				Ω(factory.ReconcileORC(ctx)).Should(Succeed())
				Expect(getCCond(
					cluster.Status.Conditions, api.ClusterConditionFailoverAck).Status).To(
					Equal(core.ConditionTrue))
			})

			It("should have pending recoveries but cluster not ready enough", func() {
				orcClient.AddInstance("asd.default", "asd-mysql-0.asd-mysql.default",
					true, -1, false)
				orcClient.AddRecoveries("asd.default", 111, false)
				cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionTrue, "", "")
				Ω(factory.ReconcileORC(ctx)).Should(Succeed())
				Expect(getCCond(
					cluster.Status.Conditions, api.ClusterConditionFailoverAck).Status).To(
					Equal(core.ConditionTrue))
				Expect(orcClient.CheckAck(111)).To(Equal(false))
			})

			It("should have pending recoveries that will be recovered", func() {
				orcClient.AddInstance("asd.default", "asd-mysql-0.asd-mysql.default",
					true, -1, false)
				orcClient.AddRecoveries("asd.default", 112, false)
				min20, _ := time.ParseDuration("-20m")
				cluster.Status.Conditions = []api.ClusterCondition{
					api.ClusterCondition{
						Type:               api.ClusterConditionReady,
						Status:             core.ConditionTrue,
						LastTransitionTime: meta.NewTime(time.Now().Add(min20)),
					},
				}

				Ω(factory.ReconcileORC(ctx)).Should(Succeed())
				Expect(getCCond(
					cluster.Status.Conditions, api.ClusterConditionFailoverAck).Status).To(
					Equal(core.ConditionTrue))
				Expect(orcClient.CheckAck(112)).To(Equal(true))
			})

		})
	})
})

func getCCond(conds []api.ClusterCondition, cType api.ClusterConditionType) *api.ClusterCondition {
	for _, c := range conds {
		if c.Type == cType {
			return &c
		}
	}
	return nil
}
