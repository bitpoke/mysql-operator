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
	"strings"
	"time"

	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

const (
	// recoveryGraceTime is the time, in seconds, that has to pass since cluster
	// is marked as Ready and to acknowledge a recovery for a cluster
	recoveryGraceTime            = 600
	defaultMaxSlaveLatency int64 = 30
	mysqlPort                    = 3306
)

type orcUpdater struct {
	cluster   *mysqlcluster.MysqlCluster
	recorder  record.EventRecorder
	orcClient orc.Interface
}

// NewOrcUpdater returns a syncer that updates cluster status from orchestrator.
func NewOrcUpdater(cluster *mysqlcluster.MysqlCluster, r record.EventRecorder, orcClient orc.Interface) syncer.Interface {
	return &orcUpdater{
		cluster:   cluster,
		recorder:  r,
		orcClient: orcClient,
	}
}

func (ou *orcUpdater) GetObject() interface{}   { return nil }
func (ou *orcUpdater) GetOwner() runtime.Object { return ou.cluster }
func (ou *orcUpdater) Sync(ctx context.Context) (syncer.SyncResult, error) {
	// get instances from orchestrator
	var (
		instances    InstancesSet
		err          error
		recoveries   []orc.TopologyRecovery
		maintenances []orc.Maintenance
	)

	if instances, err = ou.orcClient.Cluster(ou.cluster.GetClusterAlias()); err != nil {
		log.V(-1).Info("can't get instances from orchestrator", "alias", ou.cluster.GetClusterAlias(), "error", err.Error())
	}

	if len(instances) != 0 {
		if maintenances, err = ou.orcClient.Maintenance(); err != nil {
			log.V(-1).Info("can't get mainanaces from orchestrator", "alias", ou.cluster.GetClusterAlias(), "error", err.Error())
		}
	}

	// register nodes in orchestrator if needed, or remove nodes from status
	instances = ou.updateNodesInOrc(instances)

	// remove nodes that are not in orchestrator
	ou.removeNodeConditionNotInOrc(instances)

	// set readonly in orchestrator if needed
	if err = ou.markReadOnlyNodesInOrc(instances, maintenances); err != nil {
		log.Error(err, "error setting Master readOnly/writable", "instances", instances)
	}
	// update cluster status accordingly with orchestrator
	ou.updateStatusFromOrc(instances)

	// get recoveries for this cluster
	if recoveries, err = ou.orcClient.AuditRecovery(ou.cluster.GetClusterAlias()); err != nil {
		log.V(-1).Info("can't get recoveries from orchestrator", "alias", ou.cluster.GetClusterAlias(), "error", err.Error())
	}

	// update cluster status
	ou.updateStatusForRecoveries(recoveries)

	// filter recoveries that can be acknowledged
	toAck := ou.getRecoveriesToAck(recoveries)

	// acknowledge recoveries
	if err = ou.acknowledgeRecoveries(toAck); err != nil {
		log.Error(err, "failed to acknowledge recoveries", "alias", ou.cluster.GetClusterAlias(), "ack_recoveries", toAck)
	}

	return syncer.SyncResult{}, nil
}

// nolint: gocyclo
func (ou *orcUpdater) updateStatusFromOrc(insts InstancesSet) {
	log.V(1).Info("updating nodes status", "insts", insts)

	// we assume that cluster is in ReadOnly
	isReadOnly := true

	// get maxSlaveLatency for this cluster
	maxSlaveLatency := defaultMaxSlaveLatency
	if ou.cluster.Spec.MaxSlaveLatency != nil {
		maxSlaveLatency = *ou.cluster.Spec.MaxSlaveLatency
	}

	master := insts.DetermineMaster()

	// update conditions for every node
	for _, node := range insts {
		host := node.Key.Hostname

		// nodes that are not up to date in orchestrator should be marked as unknown
		if !node.IsUpToDate {
			if !node.IsLastCheckValid {
				ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionUnknown)
				ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionUnknown)
				ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionUnknown)
			}
			continue
		}

		// set node Lagged conditions
		if !node.SlaveLagSeconds.Valid {
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionUnknown)
		} else if node.SlaveLagSeconds.Int64 <= maxSlaveLatency {
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionFalse)
		} else { // node is behind master
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionTrue)
		}

		// set node replicating condition
		if node.Slave_SQL_Running && node.Slave_IO_Running {
			ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionFalse)
		}

		// set masters condition on node
		if master != nil && host == master.Key.Hostname {
			ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionFalse)
		}

		// set node read only
		if node.ReadOnly {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionFalse)
		}

		// check if cluster is read only
		isReadOnly = isReadOnly && node.ReadOnly
	}

	// set cluster ReadOnly condition
	if isReadOnly {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionTrue, "ClusterReadOnlyTrue", "cluster is in read only")
	} else {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionFalse, "ClusterReadOnlyFalse", "cluster is writable")
	}
}

// updateNodesInOrc is the functions that tries to register
// unregistered nodes and to remove nodes that does not exists.
func (ou *orcUpdater) updateNodesInOrc(instances InstancesSet) InstancesSet {
	var (
		// hosts that should be discovered
		shouldDiscover []string
		// list of instances from orchestrator that should be removed
		shouldForget []string
		// the list of instances that are in orchestrator and k8s
		instancesFiltered InstancesSet
	)

	log.V(1).Info("nodes (un)registrations", "readyNodes", ou.cluster.Status.ReadyNodes)
	log.V(2).Info("instances", "instances", instances)

	for i := 0; i < int(*ou.cluster.Spec.Replicas); i++ {
		host := ou.cluster.GetPodHostname(i)
		if inst := instances.GetInstance(host); inst == nil {
			// if index node is bigger than total ready nodes than should not be
			// added in discover list because maybe pod is not created yet
			if i < ou.cluster.Status.ReadyNodes {
				// host is not present into orchestrator
				// register new host into orchestrator
				shouldDiscover = append(shouldDiscover, host)
			}
		} else {
			// this instance is present in both k8s and orchestrator
			instancesFiltered = append(instancesFiltered, *inst)
		}
	}

	// the only state in which a node can be removed from orchestrator
	if int(*ou.cluster.Spec.Replicas) == ou.cluster.Status.ReadyNodes {
		// remove all instances from orchestrator that does not exists in k8s
		for _, inst := range instances {
			if i := instancesFiltered.GetInstance(inst.Key.Hostname); i == nil {
				shouldForget = append(shouldForget, inst.Key.Hostname)
			}
		}
	}
	if ou.cluster.DeletionTimestamp == nil {
		ou.discoverNodesInOrc(shouldDiscover)
		ou.forgetNodesFromOrc(shouldForget)
	} else {
		log.V(1).Info("cluster is deleted - forget all nodes")
		// cluster is deleted, remove all hosts from orchestrator
		var hosts []string
		for _, i := range instances {
			hosts = append(hosts, i.Key.Hostname)
		}
		ou.forgetNodesFromOrc(hosts)
	}

	return instancesFiltered
}

func (ou *orcUpdater) discoverNodesInOrc(hosts []string) {
	log.Info("discovering hosts", "hosts", hosts)
	for _, host := range hosts {
		if err := ou.orcClient.Discover(host, mysqlPort); err != nil {
			log.Error(err, "failed to discover host with orchestrator", "host", host)
		}
	}
}

func (ou *orcUpdater) forgetNodesFromOrc(hosts []string) {
	log.Info("forgeting hosts", "hosts", hosts)
	for _, host := range hosts {
		if err := ou.orcClient.Forget(host, mysqlPort); err != nil {
			log.Error(err, "failed to forget host with orchestrator", "host", host)
		}
	}
}

func (ou *orcUpdater) getRecoveriesToAck(recoveries []orc.TopologyRecovery) []orc.TopologyRecovery {
	toAck := []orc.TopologyRecovery{}

	if len(recoveries) == 0 {
		return toAck
	}

	i, found := condIndexCluster(ou.cluster, api.ClusterConditionReady)
	if !found || ou.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		log.Info("skip acknowledging recovery since cluster is not ready yet", "cluster", ou.cluster)
		return toAck
	}

	timeSinceReady := time.Since(ou.cluster.Status.Conditions[i].LastTransitionTime.Time).Seconds()
	if timeSinceReady < recoveryGraceTime {
		log.Info("cluster not ready for acknowledge", "timeSinceReady", timeSinceReady, "threshold", recoveryGraceTime)
		return toAck
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then <recoveryGraceTime> seconds
			recoveryStartTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				log.Error(err, "time parse error", "recovery", recovery)
				continue
			}
			if time.Since(recoveryStartTime).Seconds() < recoveryGraceTime {
				// skip this recovery
				log.V(1).Info("tries to recover to soon", "recovery", recovery)
				continue
			}

			toAck = append(toAck, recovery)
		}
	}
	return toAck
}

func (ou *orcUpdater) acknowledgeRecoveries(toAck []orc.TopologyRecovery) error {
	comment := fmt.Sprintf("Statefulset '%s' is healthy for more then %d seconds",
		ou.cluster.GetNameForResource(mysqlcluster.StatefulSet), recoveryGraceTime,
	)

	// acknowledge recoveries
	for _, recovery := range toAck {
		if err := ou.orcClient.AckRecovery(recovery.Id, comment); err != nil {
			return err
		}
		ou.recorder.Event(ou.cluster, eventNormal, "RecoveryAcked",
			fmt.Sprintf("Recovery with id %d was acked.", recovery.Id))
	}

	return nil
}

func (ou *orcUpdater) updateStatusForRecoveries(recoveries []orc.TopologyRecovery) {
	var unack []orc.TopologyRecovery
	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			unack = append(unack, recovery)
		}
	}

	if len(unack) > 0 {
		msg := makeRecoveryMessage(unack)
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionTrue, "PendingFailoverAckExists", msg)
	} else {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionFalse, "NoPendingFailoverAckExists", "no pending ack")
	}
}

// nolint: unparam
func condIndexCluster(cluster *mysqlcluster.MysqlCluster, condType api.ClusterConditionType) (int, bool) {
	for i, cond := range cluster.Status.Conditions {
		if cond.Type == condType {
			return i, true
		}
	}

	return 0, false
}

// updateNodeCondition is a helper function that updates condition for a specific node
func (ou *orcUpdater) updateNodeCondition(host string, cType api.NodeConditionType, status core.ConditionStatus) {
	ou.cluster.UpdateNodeConditionStatus(host, cType, status)
}

// removeNodeConditionNotInOrc marks nodes not in orc with unknown condition
// TODO: this function should remove completly from cluster.Status.Nodes nodes
// that are no longer in orchestrator and in k8s
func (ou *orcUpdater) removeNodeConditionNotInOrc(insts InstancesSet) {
	for _, ns := range ou.cluster.Status.Nodes {
		node := insts.GetInstance(ns.Name)
		if node == nil {
			// node is NOT updated so all conditions will be marked as unknown

			ou.updateNodeCondition(ns.Name, api.NodeConditionLagged, core.ConditionUnknown)
			ou.updateNodeCondition(ns.Name, api.NodeConditionReplicating, core.ConditionUnknown)
			ou.updateNodeCondition(ns.Name, api.NodeConditionMaster, core.ConditionUnknown)
			ou.updateNodeCondition(ns.Name, api.NodeConditionReadOnly, core.ConditionUnknown)
		}
	}
}

// set a host writable just if needed
func (ou *orcUpdater) setWritableNode(inst orc.Instance) error {
	if inst.ReadOnly {
		log.Info("set node writable", "node", inst.Key.Hostname)
		return ou.orcClient.SetHostWritable(inst.Key)
	}
	return nil
}

func (ou *orcUpdater) beginNodeMaintenance(inst orc.Instance, isInMaintenance bool) error {

	if isInMaintenance {
		return nil
	}

	log.Info("set node in maintenance", "node", inst.Key.Hostname)
	return ou.orcClient.BeginMaintenance(inst.Key, "mysqlcontroller", "clusterReadOnly")

}

func (ou *orcUpdater) endNodeMaintenance(inst orc.Instance, isInMaintenance bool) error {

	if !isInMaintenance {
		return nil
	}

	log.Info("set node out of maintenance", "node", inst.Key.Hostname)
	return ou.orcClient.EndMaintenance(inst.Key)
}

// set a host read only just if needed
func (ou *orcUpdater) setReadOnlyNode(inst orc.Instance) error {
	if !inst.ReadOnly {
		log.Info("set node read only", "node", inst.Key.Hostname)
		return ou.orcClient.SetHostReadOnly(inst.Key)
	}
	return nil
}

// nolint: gocyclo
func (ou *orcUpdater) markReadOnlyNodesInOrc(insts InstancesSet, maintenances []orc.Maintenance) error {
	var err error
	master := insts.DetermineMaster()
	if master == nil {
		// master is not found
		// set cluster read only
		for _, inst := range insts {
			isInMaintenance := isInstInMaintenance(inst.Key, maintenances)

			if err = ou.beginNodeMaintenance(inst, isInMaintenance); err != nil {
				log.Error(err, "failed to begin maintenance", "instance", inst)
			}
			if err = ou.setReadOnlyNode(inst); err != nil {
				log.Error(err, "failed to set read only", "instance", inst)
			}
		}
		return nil
	}

	// master is determinated
	for _, inst := range insts {
		isInMaintenance := isInstInMaintenance(inst.Key, maintenances)

		if ou.cluster.Spec.ReadOnly {
			if err = ou.beginNodeMaintenance(inst, isInMaintenance); err != nil {
				log.Error(err, "failed to begin maintenance", "instance", inst)
			}
			if err = ou.setReadOnlyNode(inst); err != nil {
				log.Error(err, "failed to set read only", "instance", inst)
			}
		} else {
			if err = ou.endNodeMaintenance(inst, isInMaintenance); err != nil {
				log.Error(err, "failed to end maintenance", "instance", inst)
			}
			if inst.Key.Hostname == master.Key.Hostname {
				if err = ou.setWritableNode(inst); err != nil {
					log.Error(err, "failed to set writable", "instance", inst)
				}
			} else {
				if err = ou.setReadOnlyNode(inst); err != nil {
					log.Error(err, "failed to set read only", "instance", inst)
				}
			}
		}
	}

	return nil
}

// InstancesSet type is a alias for []orc.Instance with extra utils functions
type InstancesSet []orc.Instance

// GetInstance returns orc.Instance for a given hostname
func (is InstancesSet) GetInstance(host string) *orc.Instance {
	for _, node := range is {
		if host == node.Key.Hostname {
			return &node
		}
	}

	return nil
}

func (is InstancesSet) getMasterForNode(node *orc.Instance) *orc.Instance {
	if len(node.MasterKey.Hostname) != 0 && !node.IsCoMaster {
		// get the master hostname from MasterKey if MasterKey is set
		master := is.GetInstance(node.MasterKey.Hostname)
		if master != nil {
			return is.getMasterForNode(master)
		}
		return nil
	}

	if node.IsCoMaster {
		// if it's comaster then return the other master
		master := is.GetInstance(node.MasterKey.Hostname)
		return master
	}

	return node
}

// DetermineMaster returns a orc.Instance that is master or nil if can't find master
func (is InstancesSet) DetermineMaster() *orc.Instance {
	masterForNode := []orc.Instance{}

	for _, node := range is {
		master := is.getMasterForNode(&node)
		if master == nil {
			return nil
		}
		masterForNode = append(masterForNode, *master)
	}

	if len(masterForNode) != 0 {
		masterHostName := masterForNode[0]
		for _, node := range masterForNode {
			if node.Key.Hostname != masterHostName.Key.Hostname {
				return nil
			}
		}
		return &masterHostName
	}

	return nil
}

// makeRecoveryMessage returns a string human readable for cluster recoveries
func makeRecoveryMessage(acks []orc.TopologyRecovery) string {
	texts := []string{}
	for _, a := range acks {
		texts = append(texts, fmt.Sprintf("{id: %d, uid: %s, success: %t, time: %s}",
			a.Id, a.UID, a.IsSuccessful, a.RecoveryStartTimestamp))
	}

	return strings.Join(texts, " ")
}

func isInstInMaintenance(instKey orc.InstanceKey, maintenances []orc.Maintenance) bool {
	for _, m := range maintenances {
		if m.Key == instKey {
			return true
		}
	}

	return false
}
