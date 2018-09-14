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
	"errors"
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

const (
	healtyMoreThanMinutes        = 10
	defaultMaxSlaveLatency int64 = 30
	mysqlPort                    = 3306
)

type orcUpdater struct {
	cluster   *wrapcluster.MysqlCluster
	recorder  record.EventRecorder
	orcClient orc.Interface
}

// Syncer interface defines Sync method
type Syncer interface {
	Sync() error
}

// NewOrcUpdater returns a syncer that updates cluster status from orchestrator.
func NewOrcUpdater(cluster *api.MysqlCluster, r record.EventRecorder, orcClient orc.Interface) Syncer {
	return &orcUpdater{
		cluster:   wrapcluster.NewMysqlClusterWrapper(cluster),
		recorder:  r,
		orcClient: orcClient,
	}
}

func (ou *orcUpdater) Sync() error {
	// get instances from orchestrator
	var (
		instances  InstancesSet
		err        error
		recoveries []orc.TopologyRecovery
	)

	if instances, err = ou.orcClient.Cluster(ou.cluster.GetClusterAlias()); err != nil {
		log.Error(err, "Can't get instances from orchestrator", "alias", ou.cluster.GetClusterAlias(), "cluster", ou.cluster)
	}

	if len(instances) == 0 {
		log.Info("No instances in orchestrator!", "clusterAlias", ou.cluster.GetClusterAlias(), "cluster", ou.cluster)
	}

	// register nodes in orchestrator if needed, or remove nodes from status
	if instances, err = ou.registerUnregisterNodesInOrc(instances); err != nil {
		log.Error(err, "Failed registering nodes into orchestrator", "cluster", ou.cluster)
	}
	// set readonly in orchestrator if needed
	if err = ou.updateNodesReadOnlyFlagInOrc(instances); err != nil {
		log.Error(err, "Error setting Master readOnly/writable", "instances", instances, "cluster", ou.cluster)
	}
	// update cluster status accordingly with orchestrator
	ou.updateStatusFromOrc(instances)

	// get reecoveries for this cluster
	if recoveries, err = ou.orcClient.AuditRecovery(ou.cluster.GetClusterAlias()); err != nil {
		log.Error(err, "Can't get recoveries from orchestrator", "alias", ou.cluster.GetClusterAlias(), "cluster", ou.cluster)
	}
	// update cluster status
	ou.updateStatusForRecoveries(recoveries)
	// filter recoveries that can be acknowledged
	toAck := ou.getRecoveriesToAck(recoveries)
	// acknowledge recoveries
	if err = ou.acknowledgeRecoveries(toAck); err != nil {
		log.Error(err, "Failed to acknowledge recoveries", "cluster", ou.cluster, "ack_recoveries", toAck)
	}
	return nil
}

// nolint: gocyclo
func (ou *orcUpdater) updateStatusFromOrc(insts InstancesSet) {
	// TODO: improve this code by computing differences between what
	// orchestartor knows and what we know

	// we assume that cluster is in ReadOnly
	isReadOnly := true

	// get maxSlaveLatency for this cluster
	maxSlaveLatency := defaultMaxSlaveLatency
	if ou.cluster.Spec.MaxSlaveLatency != nil {
		maxSlaveLatency = *ou.cluster.Spec.MaxSlaveLatency
	}

	// nodes that where updated
	updatedNodes := []string{}

	// update conditions for every node
	for _, node := range insts {
		host := node.Key.Hostname
		updatedNodes = append(updatedNodes, host)

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

		// set masters on false
		ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionFalse)

		// set node read only
		if node.ReadOnly {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionFalse)
		}

		// check if cluster is read only
		isReadOnly = isReadOnly && node.ReadOnly
	}

	// set master node Master condition on True
	master := insts.DetermineMaster()
	if master == nil {
		log.Error(nil, "Error acquiring master name", "instances", insts)
	} else {
		ou.updateNodeCondition(master.Key.Hostname, api.NodeConditionMaster, core.ConditionTrue)
	}

	// set cluster ReadOnly condition
	if isReadOnly {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionTrue, "initializedTrue", "settingReadOnlyTrue")
	} else {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionFalse, "initializedFalse", "settingReadOnlyFalse")
	}

	ou.removeNodeConditionNotIn(updatedNodes)
}

// registerUnregisterNodesInOrc is the functions that tries to register
// unregistered nodes and to remove nodes that does not exists.
func (ou *orcUpdater) registerUnregisterNodesInOrc(instSet InstancesSet) (InstancesSet, error) {
	var (
		err       error
		errMsg    string
		instances InstancesSet
	)

	for i := 0; i < ou.cluster.Status.ReadyNodes; i++ {
		host := ou.cluster.GetPodHostname(i)
		if inst := instSet.GetInstance(host); inst == nil {
			// host is not present into orchestrator
			// register new host into orchestrator
			if err = ou.orcClient.Discover(host, mysqlPort); err != nil {
				log.V(2).Info("Failed to register %s with orchestrator", "host", host, "error", err)
				errMsg += fmt.Sprintf(" failed to register %s", host)
			}
		} else {
			instances = append(instances, *inst)
		}
	}

	// remove all instances from orchestrator that does not exists in k8s
	for _, inst := range instSet {
		if i := instances.GetInstance(inst.Key.Hostname); i == nil {
			if err = ou.orcClient.Forget(inst.Key.Hostname, inst.Key.Port); err != nil {
				log.V(2).Info("Failed to forget %s with orchestrator", "host", i.Key.Hostname, "error", err)
				errMsg += fmt.Sprintf(" failed to forget %s", inst.Key.Hostname)
			}
		}
	}

	if len(errMsg) != 0 {
		err = errors.New(errMsg)
	}
	return instances, err
}

func (ou *orcUpdater) getRecoveriesToAck(recoveries []orc.TopologyRecovery) []orc.TopologyRecovery {
	// TODO: check for recoveries that need acknowledge, by excluding already acked recoveries
	toAck := []orc.TopologyRecovery{}

	if len(recoveries) == 0 {
		return toAck
	}

	i, find := condIndexCluster(ou.cluster, api.ClusterConditionReady)
	if !find || ou.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		log.Info("[getRecoveriesToAck]: Cluster is not ready for ack.")
		return toAck
	}

	if time.Since(ou.cluster.Status.Conditions[i].LastTransitionTime.Time).Minutes() < healtyMoreThanMinutes {
		log.Info("Stateful set is not ready more then 10 minutes. Don't ack.")
		return toAck
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then <healtyMoreThanMinutes> minutes
			startTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				log.Error(err, "Can't parse time: %s for audit recovery.",
					"recovery", recovery,
				)
				continue
			}
			if time.Since(startTime).Minutes() < healtyMoreThanMinutes {
				// skip this recovery
				log.Error(nil, "Tries to recover to soon.", "recovery", recovery)
				continue
			}

			toAck = append(toAck, recovery)
		}
	}
	return toAck
}

func (ou *orcUpdater) acknowledgeRecoveries(toAck []orc.TopologyRecovery) error {
	comment := fmt.Sprintf("Statefulset '%s' is healty more then 10 minutes",
		ou.cluster.GetNameForResource(api.StatefulSet),
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
		msg := getRecoveryTextMsg(unack)
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionTrue, "pendingFailoverAckExists", msg)
	} else {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionFalse, "noPendingFailoverAckExists", "no pending ack")
	}
}

// nolint: unparam
func condIndexCluster(cluster *wrapcluster.MysqlCluster, ty api.ClusterConditionType) (int, bool) {
	for i, cond := range cluster.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

func (ou *orcUpdater) updateNodeCondition(host string, cType api.NodeConditionType, status core.ConditionStatus) {
	ou.cluster.UpdateNodeConditionStatus(host, cType, status)
}

func (ou *orcUpdater) removeNodeConditionNotIn(hosts []string) {
	for _, ns := range ou.cluster.Status.Nodes {
		updated := false
		for _, h := range hosts {
			if h == ns.Name {
				updated = true
			}
		}

		if !updated {
			ou.updateNodeCondition(ns.Name, api.NodeConditionLagged, core.ConditionUnknown)
			ou.updateNodeCondition(ns.Name, api.NodeConditionReplicating, core.ConditionUnknown)
			ou.updateNodeCondition(ns.Name, api.NodeConditionMaster, core.ConditionUnknown)
		}
	}
}

// set a host writable just if needed
func (ou *orcUpdater) setInstWritable(inst orc.Instance) error {
	if inst.ReadOnly {
		log.V(2).Info(fmt.Sprintf("set instance %s writable", inst.Key.Hostname))
		return ou.orcClient.SetHostWritable(inst.Key)
	}
	return nil
}

func (ou *orcUpdater) putNodeInMaintenance(inst orc.Instance) error {

	log.V(2).Info(fmt.Sprintf("set instance %s in maintenance", inst.Key.Hostname))
	return ou.orcClient.BeginMaintenance(inst.Key, "mysqlcontroller", "clusterReadOnly")

}

func (ou *orcUpdater) getNodeOutOfMaintenance(inst orc.Instance) error {

	log.V(2).Info(fmt.Sprintf("set instance %s out of maintenance", inst.Key.Hostname))
	return ou.orcClient.EndMaintenance(inst.Key)

}

// set a host read only just if needed
func (ou *orcUpdater) setInstReadOnly(inst orc.Instance) error {
	if !inst.ReadOnly {
		log.V(2).Info(fmt.Sprintf("set instance %s read only", inst.Key.Hostname))
		return ou.orcClient.SetHostReadOnly(inst.Key)
	}
	return nil
}

// nolint: gocyclo
func (ou *orcUpdater) updateNodesReadOnlyFlagInOrc(insts InstancesSet) error {
	var err error
	master := insts.DetermineMaster()
	if master == nil {
		// master is not found
		// set cluster read only
		for _, inst := range insts {
			if err = ou.putNodeInMaintenance(inst); err != nil {
				log.Error(err, "Put node in maintenance")
			}
			if err = ou.setInstReadOnly(inst); err != nil {
				log.Error(err, "Put node in read only")
			}
		}
		return nil
	}

	// master is determinated
	for _, inst := range insts {
		if ou.cluster.Spec.ReadOnly {
			if err = ou.putNodeInMaintenance(inst); err != nil {
				log.Error(err, "Put node in maintenance")
			}
			if err = ou.setInstReadOnly(inst); err != nil {
				log.Error(err, "Put node in read only")
			}
		} else if !ou.cluster.Spec.ReadOnly {
			if err = ou.getNodeOutOfMaintenance(inst); err != nil {
				log.Error(err, "Get node out of maintenance")
			}
			if inst.Key.Hostname == master.Key.Hostname {
				if err = ou.setInstWritable(inst); err != nil {
					log.Error(err, "Set node as writable")
				}
			} else {
				if err = ou.setInstReadOnly(inst); err != nil {
					log.Error(err, "Put node in read only")
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
		return is.getMasterForNode(master)
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
		check := true
		for _, node := range masterForNode {
			if node.Key.Hostname != masterHostName.Key.Hostname {
				check = false
			}
		}
		if !check {
			return nil
		}
		return &masterHostName
	}

	return nil
}

// getRecoveryTextMsg returns a string human readable for cluster recoveries
func getRecoveryTextMsg(acks []orc.TopologyRecovery) string {
	text := ""
	for _, a := range acks {
		text += fmt.Sprintf(" {id: %d, uid: %s, success: %t, time: %s}",
			a.Id, a.UID, a.IsSuccessful, a.RecoveryStartTimestamp)
	}

	return fmt.Sprintf("[%s]", text)
}
