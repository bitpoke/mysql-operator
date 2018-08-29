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
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

const (
	healtyMoreThanMinutes        = 10
	defaultMaxSlaveLatency int64 = 30
	mysqlPort                    = 3306
)

type orcUpdater struct {
	cluster   *api.MysqlCluster
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
		cluster:   cluster,
		recorder:  r,
		orcClient: orcClient,
	}
}

func (ou *orcUpdater) Sync() error {
	// sync status from orchestrator
	if insts, err := ou.orcClient.Cluster(ou.cluster.GetClusterAlias()); err == nil {

		err = ou.updateNodesReadOnlyFlagInOrc(insts)
		if err != nil {
			log.Error(err, "Error setting Master readOnly/writable", "instances", insts)
		}

		ou.updateStatusFromOrc(insts)
	} else {
		log.Error(err, "Fail to get cluster from orchestrator: %s. Now tries to register nodes.")
		return ou.registerNodesInOrc()
	}

	// check cluster recoveries and ack them
	if recoveries, err := ou.orcClient.AuditRecovery(ou.cluster.GetClusterAlias()); err == nil {
		ou.updateStatusForRecoveries(recoveries)
		toAck := ou.getRecoveriesToAck(recoveries)

		comment := fmt.Sprintf("Statefulset '%s' is healty more then 10 minutes",
			ou.cluster.GetNameForResource(api.StatefulSet),
		)

		// acknowledge recoveries
		for _, recovery := range toAck {
			if err := ou.orcClient.AckRecovery(recovery.Id, comment); err != nil {
				log.Error(err, "Trying to ack recovery with id %d but failed with error",
					"recovery", recovery,
				)
			}
			ou.recorder.Event(ou.cluster, eventNormal, "RecoveryAcked",
				fmt.Sprintf("Recovery with id %d was acked.", recovery.Id))
		}
	}
	return nil
}

func (ou *orcUpdater) updateStatusFromOrc(insts []orc.Instance) {
	// TODO: imporve this code by computing differences between what
	// orchestartor knows and what we know

	updatedNodes := []string{}

	var isReadOnly bool = true
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
		maxSlaveLatency := defaultMaxSlaveLatency
		if ou.cluster.Spec.MaxSlaveLatency != nil {
			maxSlaveLatency = *ou.cluster.Spec.MaxSlaveLatency
		}
		if !node.SlaveLagSeconds.Valid {
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionUnknown)
		} else if node.SlaveLagSeconds.Int64 <= maxSlaveLatency {
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionFalse)
		} else { // node is behind master
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionTrue)
		}
		if node.Slave_SQL_Running && node.Slave_IO_Running {
			ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionFalse)
		}
		ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionFalse)
		isReadOnly = isReadOnly && node.ReadOnly
		if node.ReadOnly == true {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionTrue)
		} else {
			ou.updateNodeCondition(host, api.NodeConditionReadOnly, core.ConditionFalse)
		}
	}

	master, err := determineMasterFor(insts)
	if err != nil {
		log.Error(err, "Error acquiring master name")
	} else {
		ou.updateNodeCondition(master.Key.Hostname, api.NodeConditionMaster, core.ConditionTrue)

		if isReadOnly == true {
			ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
				core.ConditionTrue, "initializedTrue", "settingReadOnlyTrue")
		} else {
			ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
				core.ConditionFalse, "initializedFalse", "settingReadOnlyFalse")
		}
	}
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

func (ou *orcUpdater) registerNodesInOrc() error {
	// Register nodes in orchestrator
	// try to discover ready nodes into orchestrator
	for i := 0; i < int(ou.cluster.Status.ReadyNodes); i++ {
		host := ou.cluster.GetPodHostname(i)
		if err := ou.orcClient.Discover(host, mysqlPort); err != nil {
			log.Info("Failed to register %s with orchestrator", "host", host, "error", err)
		}
	}

	return nil
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
				log.Error(err, "[getRecoveriesToAck] Can't parse time: %s for audit recovery",
					"recovery", recovery,
				)
				continue
			}
			if time.Since(startTime).Minutes() < healtyMoreThanMinutes {
				// skip this recovery
				log.Error(nil, "[getRecoveriesToAck] recovery to soon", "recovery", recovery)
				continue
			}

			toAck = append(toAck, recovery)
		}
	}
	return toAck
}

// nolint: unparam
func condIndexCluster(cluster *api.MysqlCluster, ty api.ClusterConditionType) (int, bool) {
	for i, cond := range cluster.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

func (ou *orcUpdater) updateNodeCondition(host string, cType api.NodeConditionType, status core.ConditionStatus) {
	i := ou.cluster.Status.GetNodeStatusIndex(host)
	ou.cluster.Status.Nodes[i].UpdateNodeCondition(cType, status)
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

// getRecoveryTextMsg returns a string human readable for cluster recoveries
func getRecoveryTextMsg(acks []orc.TopologyRecovery) string {
	text := ""
	for _, a := range acks {
		text += fmt.Sprintf(" {id: %d, uid: %s, success: %t, time: %s}",
			a.Id, a.UID, a.IsSuccessful, a.RecoveryStartTimestamp)
	}

	return fmt.Sprintf("[%s]", text)
}

func getInstance(hostname string, insts []orc.Instance) (*orc.Instance, error) {

	for _, node := range insts {
		host := node.Key.Hostname

		if host == hostname {
			return &node, nil
		}
	}

	return nil, fmt.Errorf("the element was not found")
}

func getMaster(node *orc.Instance, insts []orc.Instance) (*orc.Instance, error) {

	if len(node.MasterKey.Hostname) != 0 && node.IsCoMaster == false {
		next, err := getInstance(node.MasterKey.Hostname, insts)
		if err == nil {
			return getMaster(next, insts)
		} else {
			return nil, err
		}
	}

	if node.IsCoMaster == true {
		next, err := getInstance(node.MasterKey.Hostname, insts)
		if err == nil {
			return next, nil
		} else {
			return nil, err
		}
	}

	return node, nil
}

func determineMasterFor(insts []orc.Instance) (*orc.Instance, error) {

	var masterForNode []orc.Instance

	for _, node := range insts {
		master, err := getMaster(&node, insts)
		if err == nil {
			masterForNode = append(masterForNode, *master)
		} else {
			return nil, fmt.Errorf("not able to retrieve the root of this node %s", node.Key.Hostname)
		}
	}

	if len(masterForNode) != 0 {
		masterHostName := masterForNode[0]
		var check bool = true
		for _, node := range masterForNode {
			if node.Key.Hostname != masterHostName.Key.Hostname {
				check = false
			}
		}
		if check == true {
			return &masterHostName, nil
		} else {
			return nil, fmt.Errorf("multiple masters")
		}
	} else {
		return nil, fmt.Errorf("0 elements in instance array")
	}

}

// set a host writable just if needed
func (ou *orcUpdater) setInstWritable(inst orc.Instance) error {
	if inst.ReadOnly == true {
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
	if !inst.ReadOnly == true {
		log.V(2).Info(fmt.Sprintf("set instance %s read only", inst.Key.Hostname))
		return ou.orcClient.SetHostReadOnly(inst.Key)
	}
	return nil
}

func (ou *orcUpdater) updateNodesReadOnlyFlagInOrc(insts []orc.Instance) error {
	master, err := determineMasterFor(insts)
	if err != nil && err.Error() == "multiple masters" {
		// master is not found
		// set cluster read only
		for _, inst := range insts {
			ou.putNodeInMaintenance(inst)
			ou.setInstReadOnly(inst)
		}
		return nil
	} else if err != nil {
		return err
	}

	// master is determinated
	for _, inst := range insts {
		if ou.cluster.Spec.ReadOnly == true {
			ou.putNodeInMaintenance(inst)
			ou.setInstReadOnly(inst)
		} else if ou.cluster.Spec.ReadOnly == false && err == nil {
			ou.getNodeOutOfMaintenance(inst)
			if inst.Key.Hostname == master.Key.Hostname {
				ou.setInstWritable(inst)
			} else {
				ou.setInstReadOnly(inst)
			}
		}
	}

	return nil
}
