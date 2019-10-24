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
	"database/sql"
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
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace},
			Status: api.MysqlClusterStatus{
				ReadyNodes: 1,
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   &one,
				SecretName: clusterKey.Name,
			},
		})

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

	When("orchestrator is not available", func() {
		BeforeEach(func() {
			// register nodes into orchestrator
			cluster.Status.ReadyNodes = 1
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// second reconcile event to update cluster status
			_, err = orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// check that sync was successful
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(
				haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))

			// make orchestrator fake client unreachable
			orcClient.MakeOrcUnreachable()
		})

		It("should not reconcile and keep the last known state", func() {
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).ToNot(Succeed())

			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
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
				SlaveLagSeconds: sql.NullInt64{
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
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReadOnly, core.ConditionFalse, "ClusterReadOnlyFalse"))
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionFalse, "NoPendingFailoverAckExists"))
			Expect(cluster.Status.Nodes).To(HaveLen(1))
		})

		It("should mark cluster having pending recoveries", func() {
			// AddRecoveries signature: cluster, acked
			orcClient.AddRecoveries(cluster.GetClusterAlias(), false)
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue, "PendingFailoverAckExists"))
		})

		It("should not acknowledge pending recoveries when cluster is not ready for enough time", func() {
			// AddRecoveries signature: cluster, acked
			id := orcClient.AddRecoveries(cluster.GetClusterAlias(), false)

			// mark cluster as ready
			cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionTrue, "", "")
			_, err := orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue, "PendingFailoverAckExists"))
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
			Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue, "PendingFailoverAckExists"))
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
			cluster.Spec.Replicas = &two
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
			cluster.Spec.Replicas = &one
			cluster.Status.ReadyNodes = 1

			// sync
			_, err = orcSyncer.Sync(context.TODO())
			Expect(err).To(Succeed())

			// check the node 1 should not be in the list
			Expect(cluster.Status.Nodes).To(HaveLen(1))

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

	It("should not determine the master when master is not in orc", func() {
		orcClient.AddInstance(orc.Instance{
			ClusterName: cluster.GetClusterAlias(),
			Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
			MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
		})

		var insts InstancesSet
		insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
		Expect(insts).To(HaveLen(1))

		// should not determine any master because there are two masters
		master := insts.DetermineMaster()
		Expect(master).To(BeNil())
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
			// set cluster on readonly, master should be in read only state
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    false, // mark node as master
				// mark instance as uptodate
				IsUpToDate:       true,
				IsLastCheckValid: true,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(1)},
				MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
				ReadOnly:    true,
				// set replication running on replica
				Slave_SQL_Running: true,
				Slave_IO_Running:  true,
				// mark instance as uptodate
				IsUpToDate:       true,
				IsLastCheckValid: true,
			})

			// update cluster nodes status
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			master, _ := orcClient.Master(cluster.GetClusterAlias())
			updater.updateStatusFromOrc(insts, master)
		})

		It("should update status for nodes on cluster", func() {
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
			Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionReplicating, core.ConditionTrue))
		})

		It("should set the master readOnly when cluster is read only", func() {
			cluster.Spec.ReadOnly = true

			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			master, _ := orcClient.Master(cluster.GetClusterAlias())
			updater.markReadOnlyNodesInOrc(insts, master)

			// check master (node-0) to be read-only
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			node0 := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
			Expect(node0.ReadOnly).To(Equal(true))
		})

		It("should set the master writable when cluster is writable", func() {
			//Set ReadOnly to false in order to get the master Writable
			cluster.Spec.ReadOnly = false

			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			master := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0)) // set node0 as master
			updater.markReadOnlyNodesInOrc(insts, master)

			// check master (node-0) to be writable
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			node0 := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
			Expect(node0.ReadOnly).To(Equal(false))

			// check slave (node-1) to be read-only
			node1 := InstancesSet(insts).GetInstance(cluster.GetPodHostname(1))
			Expect(node1.ReadOnly).To(Equal(true))
		})

		It("should not set the master writable during failover", func() {
			// Get master
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			master := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))

			// Simulate failover status:
			//  1. Orchestrator would have set read-only on the master, and
			//  2. ClusterConditionFailoverInProgress would be set to true
			updater.setReadOnlyNode(*master)
			cluster.UpdateStatusCondition(api.ClusterConditionFailoverInProgress, core.ConditionTrue,
				"TestFailover", "Failover is in progress")

			// check master (node-0) to be read-only before
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			node0 := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
			Expect(node0.ReadOnly).To(Equal(true))

			updater.markReadOnlyNodesInOrc(insts, master)

			// check master (node-0) to be read-only after
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			node0 = InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
			Expect(node0.ReadOnly).To(Equal(true))

			// check slave (node-1) to be read-only
			node1 := InstancesSet(insts).GetInstance(cluster.GetPodHostname(1))
			Expect(node1.ReadOnly).To(Equal(true))
		})

		It("should remove old nodes from orchestrator", func() {
			cluster.Spec.Replicas = &one
			cluster.Status.ReadyNodes = 1
			// set cluster ready condition and set lastTransitionTime to 100 seconds before
			cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionTrue, "", "")
			ltt := metav1.NewTime(time.Now().Add(-100 * time.Second))
			cluster.GetClusterCondition(api.ClusterConditionReady).LastTransitionTime = ltt

			// call register and unregister nodes in orc
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			_, _, rm := updater.updateNodesInOrc(insts)
			updater.forgetNodesFromOrc(rm)

			// check for instances in orc
			insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
			Expect(insts).To(HaveLen(1))
		})

		It("should remove nodes from cluster status at scale down", func() {
			// scale down the cluster
			cluster.Spec.Replicas = &one
			cluster.Status.ReadyNodes = 1

			// call the function
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			updater.removeNodeConditionNotInOrc(insts)

			Expect(cluster.Status.Nodes).To(ConsistOf(hasNodeWithStatus(cluster.GetPodHostname(0))))
		})

		When("cluster is not ready", func() {
			BeforeEach(func() {
				cluster.Spec.Replicas = &two
				cluster.Status.ReadyNodes = 1
				// call status updater to update status
				updater.updateClusterReadyStatus()
			})

			It("should mark cluster as not ready", func() {
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReady, core.ConditionFalse, "StatefulSetNotReady"))

				// mark cluster as ready
				cluster.Status.ReadyNodes = 2
				updater.updateClusterReadyStatus()

				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReady, core.ConditionTrue, "ClusterReady"))

				// update cluster nodes status but this time with a not replicating node
				orcClient.AddInstance(orc.Instance{
					ClusterName: cluster.GetClusterAlias(),
					Key:         orc.InstanceKey{Hostname: cluster.GetPodHostname(2)},
					MasterKey:   orc.InstanceKey{Hostname: cluster.GetPodHostname(0)},
					ReadOnly:    true,
					// set replication running on replica
					Slave_SQL_Running: false,
					Slave_IO_Running:  false,
					// mark instance as uptodate
					IsUpToDate:       true,
					IsLastCheckValid: true,
				})
				insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
				master, _ := orcClient.Master(cluster.GetClusterAlias())
				cluster.Spec.Replicas = &three
				cluster.Status.ReadyNodes = 3
				updater.updateStatusFromOrc(insts, master)
				updater.updateClusterReadyStatus()

				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReady, core.ConditionFalse, "NotReplicating"))
			})

			It("should not remove nodes from orchestrator", func() {
				// call register and unregister nodes in orc
				insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
				_, _, rm := updater.updateNodesInOrc(insts)
				updater.forgetNodesFromOrc(rm)

				// check for instances in orc
				insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
				Expect(insts).To(HaveLen(2))
			})
		})

	})

	// NOTE: this test sute should be deleted in next major version
	Describe("status updater unit tests at upgrade", func() {
		var (
			updater *orcUpdater
		)

		BeforeEach(func() {
			updater = &orcUpdater{
				cluster:   cluster,
				recorder:  rec,
				orcClient: orcClient,
			}
			// set cluster on readonly, master should be in read only state
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: oldPodHostname(cluster, 0)},
				ReadOnly:    false, // mark node as master
				// mark instance as uptodate
				IsUpToDate:       true,
				IsLastCheckValid: true,
			})
			orcClient.AddInstance(orc.Instance{
				ClusterName: cluster.GetClusterAlias(),
				Key:         orc.InstanceKey{Hostname: oldPodHostname(cluster, 1)},
				MasterKey:   orc.InstanceKey{Hostname: oldPodHostname(cluster, 0)},
			})

			// update cluster nodes status
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			master, _ := orcClient.Master(cluster.GetClusterAlias())
			updater.updateStatusFromOrc(insts, master)
		})

		It("should not remove nodes from cluster when upgrading", func() {
			// scale down the cluster
			cluster.Spec.Replicas = &one
			cluster.Status.ReadyNodes = 1

			// call the function
			insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
			updater.removeNodeConditionNotInOrc(insts)

			// nothing should be remove
			Expect(cluster.Status.Nodes).To(ConsistOf(hasNodeWithStatus(oldPodHostname(cluster, 0))))
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
func haveCondWithStatus(condType api.ClusterConditionType, status core.ConditionStatus, reason string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(condType),
			"Status": Equal(status),
			"Reason": Equal(reason),
		})),
	})
}

func hasNodeWithStatus(host string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Name": Equal(host),
	})
}

func oldPodHostname(cluster *mysqlcluster.MysqlCluster, index int) string {
	return fmt.Sprintf("%s-mysql-%d.%s.%s", cluster.Name, index, cluster.GetNameForResource(mysqlcluster.OldHeadlessSVC),
		cluster.Namespace)
}
