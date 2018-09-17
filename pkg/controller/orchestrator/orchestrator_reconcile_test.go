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

package orchestratormysql

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
	fakeOrc "github.com/presslabs/mysql-operator/pkg/orchestrator/fake"
)

var _ = Describe("MysqlCluster controller", func() {
	var (
		cluster   *wrapcluster.MysqlCluster
		orcClient *fakeOrc.OrcFakeClient
		rec       *record.FakeRecorder
		orcSyncer Syncer
	)

	BeforeEach(func() {
		rec = record.NewFakeRecorder(100)
		orcClient = fakeOrc.New()
		theCluster := &api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "the-name", Namespace: "the-namespace"},
			Status: api.MysqlClusterStatus{
				ReadyNodes: 1,
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   1,
				SecretName: "a-name",
			},
		}
		orcSyncer = NewOrcUpdater(theCluster, rec, orcClient)
		cluster = wrapcluster.NewMysqlClusterWrapper(theCluster)
	})

	Describe("Update status from orc", func() {
		When("cluster does not exists in orc", func() {
			It("should register into orc", func() {
				cluster.Status.ReadyNodes = 1
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(orcClient.CheckDiscovered(cluster.GetPodHostname(0))).To(Equal(true))
			})

			It("should update status", func() {

				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				orcClient.AddRecoveries(cluster.GetClusterAlias(), 1, true)

				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionReadOnly, core.ConditionFalse))
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionFalse))
			})

			It("should have pending recoveries", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				orcClient.AddRecoveries(cluster.GetClusterAlias(), 11, false)
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
			})

			It("should have pending recoveries but cluster not ready enough", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				orcClient.AddRecoveries(cluster.GetClusterAlias(), 111, false)
				cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionTrue, "", "")
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
				Expect(orcClient.CheckAck(111)).To(Equal(false))
			})

			It("should have pending recoveries that will be recovered", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				orcClient.AddRecoveries(cluster.GetClusterAlias(), 112, false)
				min20, _ := time.ParseDuration("-20m")
				cluster.Status.Conditions = []api.ClusterCondition{
					api.ClusterCondition{
						Type:               api.ClusterConditionReady,
						Status:             core.ConditionTrue,
						LastTransitionTime: metav1.NewTime(time.Now().Add(min20)),
					},
				}

				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.Status).To(haveCondWithStatus(api.ClusterConditionFailoverAck, core.ConditionTrue))
				Expect(orcClient.CheckAck(112)).To(Equal(true))

				var event string
				Expect(rec.Events).To(Receive(&event))
				Expect(event).To(ContainSubstring("RecoveryAcked"))
			})

			It("master is in orc", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, false)
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
			})

			It("node not in orc", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))

				orcClient.RemoveInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0))
				Expect(orcSyncer.Sync()).To(Succeed())
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionUnknown))
			})

			It("existence of a single master", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				inst := orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: "", Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(1),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(0), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(2),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(0), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(3),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(2), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(4),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(3), Port: 3306}

				// Topology:
				// 0 < - 1
				//    \ - 2 < - 3 < - 4

				var insts InstancesSet
				insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

				// should determin the master as 0
				master := insts.DetermineMaster()
				Expect(master.Key.Hostname).To(Equal(cluster.GetPodHostname(0)))
			})

			It("existence of multiple masters", func() {
				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				inst := orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(5), Port: 3306}
				inst.IsCoMaster = true

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(1),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(0), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(2),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(0), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(3),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(2), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(4),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(5), Port: 3306}

				inst = orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(5),
					false, -1, false, true)
				inst.Key.Port = 3306
				inst.MasterKey = orc.InstanceKey{Hostname: cluster.GetPodHostname(0), Port: 3306}
				inst.IsCoMaster = true

				// Topology:
				//  0 <- 1
				//  |  \_ 2 <- 3
				//  5 <- 4

				var insts InstancesSet
				insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

				// should not determin any master because there are two masters
				master := insts.DetermineMaster()
				Expect(master).To(BeNil())

			})

			It("no instances", func() {
				var insts InstancesSet
				insts, _ = orcClient.Cluster(cluster.GetClusterAlias())

				master := insts.DetermineMaster()
				Expect(master).To(BeNil())
			})

			Describe("orc updater unit tests", func() {
				var (
					updater *orcUpdater
				)

				BeforeEach(func() {
					updater = &orcUpdater{
						cluster:   wrapcluster.NewMysqlClusterWrapper(cluster.MysqlCluster),
						recorder:  rec,
						orcClient: orcClient,
					}
				})
				It("set master readOnly/Writable", func() {
					//Set ReadOnly to true in order to get master ReadOnly
					//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
					orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
						true, -1, false, true)

					// set cluster on readonly, master should be in read only state
					cluster.Spec.ReadOnly = true

					insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
					Expect(updater.markReadOnlyNodeInOrc(insts)).To(Succeed())

					insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
					for _, instance := range insts {
						if instance.Key.Hostname == cluster.GetPodHostname(0) {
							Expect(instance.ReadOnly).To(Equal(true))
						}
					}

					//Set ReadOnly to false in order to get the master Writable
					cluster.Spec.ReadOnly = false

					insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
					Expect(updater.markReadOnlyNodeInOrc(insts)).To(Succeed())

					insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
					master := InstancesSet(insts).GetInstance(cluster.GetPodHostname(0))
					Expect(master.ReadOnly).To(Equal(false))

				})

				It("should remove from orc nodes that does not exists anymore", func() {
					//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
					orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
						true, -1, false, true)
					orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(1),
						true, -1, false, true)

					// call register and unregister nodes in orc
					insts, _ := orcClient.Cluster(cluster.GetClusterAlias())
					_, err := updater.registerUnregisterNodesInOrc(insts)
					Expect(err).To(Succeed())

					// check for instances in orc
					insts, _ = orcClient.Cluster(cluster.GetClusterAlias())
					Expect(insts).To(HaveLen(1))
				})
			})

			It("should mark nodes as unknown when scale down", func() {
				cluster.Spec.Replicas = 2
				cluster.Status.ReadyNodes = 2

				//AddInstance signature: cluster, host string, master bool, lag int64, slaveRunning, upToDate bool
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(0),
					true, -1, false, true)
				orcClient.AddInstance(cluster.GetClusterAlias(), cluster.GetPodHostname(1),
					false, -1, true, true)

				// sync
				Expect(orcSyncer.Sync()).To(Succeed())

				// scale down the cluster
				cluster.Spec.Replicas = 1
				cluster.Status.ReadyNodes = 1

				// sync
				Expect(orcSyncer.Sync()).To(Succeed())

				// check for conditions on node 1 to be unknown
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionUnknown))
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionLagged, core.ConditionUnknown))
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(1))).To(haveNodeCondWithStatus(api.NodeConditionReadOnly, core.ConditionUnknown))

				// node 0 should be ok
				Expect(cluster.GetNodeStatusFor(cluster.GetPodHostname(0))).To(haveNodeCondWithStatus(api.NodeConditionMaster, core.ConditionTrue))
			})
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
