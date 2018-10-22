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

package orchestrator

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
	fakeOrc "github.com/presslabs/mysql-operator/pkg/orchestrator/fake"
)

var _ = Describe("Orchestrator reconciler", func() {
	var (
		cluster   *mysqlcluster.MysqlCluster
		orcClient *fakeOrc.OrcFakeClient
		rec       *record.FakeRecorder
		orcSyncer syncer.Interface
	)

	BeforeEach(func() {
		clusterKey := types.NamespacedName{
			Name:      fmt.Sprintf("cluster-%d", rand.Int31()),
			Namespace: "default",
		}

		rec = record.NewFakeRecorder(100)
		orcClient = fakeOrc.New()
		theCluster := &api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
			Status: api.MysqlClusterStatus{
				ReadyNodes: 1,
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   1,
				SecretName: clusterKey.Name,
			},
		}

		cluster = mysqlcluster.New(theCluster)
		orcSyncer = NewOrcUpdater(cluster, rec, orcClient)
	})

	When("cluster does not exists in orchestrator", func() {
		It("should register into orchestrator", func() {
			cluster.Status.ReadyNodes = 1
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(orcClient.CheckDiscovered(cluster.GetPodHostname(0))).To(Equal(true))
		})
	})

	When("cluster is registered in orchestrator", func() {
		BeforeEach(func() {
			// AddRecoveries signature: cluster, acked
			orcClient.AddRecoveries(cluster.GetClusterAlias(), true)
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    false,
				SlaveLagSeconds: orc.NullInt64{
					Valid: false,
					Int64: 0,
				},
				Slave_SQL_Running: false,
				Slave_IO_Running:  false,
				IsUpToDate:        true,
				IsLastCheckValid:  true,
			})
		})

		It("should update cluster status", func() {
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReadOnly, core.ConditionFalse))
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionFalse))
			Expect(cluster.Status.Nodes).To(HaveLen(1))
		})

		It("should mark cluster having pending recoveries", func() {
			// AddRecoveries signature: cluster, acked
			orcClient.AddRecoveries(cluster.GetClusterAlias(), false)
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
		})

		It("should not acknowledge pending recoveries when cluster is not ready for enough time", func() {
			// AddRecoveries signature: cluster, acked
			id := orcClient.AddRecoveries(cluster.GetClusterAlias(), false)

			// mark cluster as ready
			cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionTrue, "", "")
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
			Expect(orcClient.CheckAck(id)).To(Equal(false))
		})

		It("should acknowledge pending recoveries after a grace period", func() {
			// AddRecoveries signature: cluster, id, acked
			id := orcClient.AddRecoveries(cluster.GetClusterAlias(), false)
			min20, _ := time.ParseDuration("-20m")
			cluster.Status.Conditions = []api.ClusterCondition{
				{
					Type:               api.ClusterConditionReady,
					Status:             core.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(min20)),
				},
			}

			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
			Expect(orcClient.CheckAck(id)).To(Equal(true))

			var event string
			Expect(rec.Events).To(Receive(&event))
			Expect(event).To(ContainSubstring("RecoveryAcked"))
		})

		It("should mark master in cluster nodes status", func() {
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
		})

		It("should remove master status when node is unregistered from orchestrator", func() {
			// sync once to update cluster status
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// remove node from orchestrator
			orcClient.RemoveInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0))

			// sync again
			_, err = orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// check for node status
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionUnknown))
		})

		It("should mark nodes as unknown when scale down", func() {
			cluster.Spec.Replicas = 2
			cluster.Status.ReadyNodes = 2

			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    false,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
			})

			// sync
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// scale down the cluster
			cluster.Spec.Replicas = 1
			cluster.Status.ReadyNodes = 1

			// sync
			_, err = orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// check for conditions on node 1 to be unknown
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionUnknown))
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionLagged, core.ConditionUnknown))
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionReadOnly, core.ConditionUnknown))

			// node 0 should be ok
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
		})
	})

	When("topology has a single master", func() {
		It("should successfully find the master", func() {
			// Topology:
			// 0 < - 1
			//    \ - 2 < - 3

			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(2)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(3)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(2)},
			})
			var insts InstancesSet
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

			// should determine the master as 0
			master := insts.DetermineMaster()
			Expect(master.Key.Hostname).To(Equal(cluster.GetPodHostname(0)))
		})
	})

	When("topology has co-master", func() {
		It("should be unable to find a clear master", func() {
			// Topology:
			//  0 <- 1
			//  |  \_ 2 <- 3
			//  5 <- 4
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(5)},
				IsCoMaster:  true,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(2)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(3)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(2)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(4)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(5)},
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(5)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				IsCoMaster:  true,
			})
			var insts InstancesSet
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

			// should not determine any master because there are two masters
			master := insts.DetermineMaster()
			Expect(master).To(BeNil())

		})
	})

	When("there is no node registered in orchestrator", func() {
		It("should be unable to find a master", func() {
			var insts InstancesSet
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

			master := insts.DetermineMaster()
			Expect(master).To(BeNil())
		})
	})

	Describe("status updater unit tests", func() {
		var (
			updater *orcUpdater
		)

		BeforeEach(func() {
			updater = &orcUpdater{
				cluster:   cluster,
				recorder:  rec,
				orcClient: orcClient,
			}

		})

		It("should set the master readOnly when cluster is read only", func() {
			// set cluster on readonly, master should be in read only state
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    false,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
			})

			cluster.Spec.ReadOnly = true

			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			Expect(updater.markReadOnlyNodesInOrc(insts)).To(Succeed())

			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			for _, instance := range insts {
				if instance.Key.Hostname == cluster.GetPodHostname(0) {
					Expect(instance.ReadOnly).To(Equal(true))
				}
			}

		})

		It("should set the master writable when cluster is writable", func() {
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
			})

			//Set ReadOnly to false in order to get the master Writable
			cluster.Spec.ReadOnly = false

			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			Expect(updater.markReadOnlyNodesInOrc(insts)).To(Succeed())

			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			master := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
			Expect(master.ReadOnly).To(Equal(false))

			slave := InstancesSet(insts).GetInstance(cluster.GetPodHostname(1))
			Expect(slave.ReadOnly).To(Equal(true))

		})

		It("should remove old nodes from orchestrator", func() {
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    false,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
			})

			cluster.Spec.Replicas = 1
			cluster.Status.ReadyNodes = 1

			// call register and unregister nodes in orc
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			updater.updateNodesInOrc(insts)

			// check for instances in orc
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			Expect(insts).To(HaveLen(1))
		})
	})
})

// haveNodeCondWithStatus is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
// nolint: unparam
func haveNodeCondWithStatus(condType api.NodeConditionType, status core.ConditionStatus) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(condType),
			"Status": Equal(status),
		})),
	})
}

// haveCondWithStatus is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
func haveCondWithStatus(condType api.ClusterConditionType, status core.ConditionStatus) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(condType),
			"Status": Equal(status),
		})),
	})

}
