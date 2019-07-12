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
	"regexp"
	"strconv"
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
	recoveryGraceTime = 600
	// forgetGraceTime represents the time, in seconds, that needs to pass since cluster is ready to
	// remove a node from orchestrator
	forgetGraceTime              = 30
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
		allInstances InstancesSet
		err          error
		recoveries   []orc.TopologyRecovery
		master       *orc.Instance
	)

	// query orchestrator for information
	if allInstances, master, err = ou.getFromOrchestrator(); err != nil {
		return syncer.SyncResult{}, err
	}

	// register nodes in orchestrator if needed, or remove nodes from status
	instances, undiscoveredInstances, toRemoveInstances := ou.updateNodesInOrc(allInstances)

	// register new nodes into orchestrator
	ou.discoverNodesInOrc(undiscoveredInstances)

	// remove nodes which are not registered in orchestrator from status
	ou.removeNodeConditionNotInOrc(instances)

	// set readonly in orchestrator if needed
	ou.markReadOnlyNodesInOrc(instances, master)

	// update cluster status accordingly with orchestrator
	ou.updateStatusFromOrc(instances, master)

	// updates cluster ready status based on nodes status from orchestrator
	ou.updateClusterReadyStatus()

	// remove old nodes from orchestrator, depends on cluster ready status
	ou.forgetNodesFromOrc(toRemoveInstances)

	// get recoveries for this cluster
	if recoveries, err = ou.orcClient.AuditRecovery(ou.cluster.GetClusterAlias()); err != nil {
		log.V(-1).Info("can't get recoveries from orchestrator", "alias", ou.cluster.GetClusterAlias(), "error", err.Error())
	}

	// update cluster status for recoveries
	ou.updateStatusForRecoveries(recoveries)

	// filter recoveries that can be acknowledged
	toAck := ou.getRecoveriesToAck(recoveries)

	// acknowledge recoveries, depends on cluster ready status
	if err = ou.acknowledgeRecoveries(toAck); err != nil {
		log.Error(err, "failed to acknowledge recoveries", "alias", ou.cluster.GetClusterAlias(), "ack_recoveries", toAck)
	}

	return syncer.SyncResult{}, nil
}

func (ou *orcUpdater) getFromOrchestrator() (instances []orc.Instance, master *orc.Instance, err error) {

	// get all related instances from orchestrator
	if instances, err = ou.orcClient.Cluster(ou.cluster.GetClusterAlias()); err != nil {
		if !orc.IsNotFound(err) {
			log.Error(err, "Orchestrator is not reachable", "cluster_alias", ou.cluster.GetClusterAlias())
			return instances, master, err
		}
		log.V(-1).Info("can't get instances from Orchestrator", "msg", "not found", "alias", ou.cluster.GetClusterAlias())
		return instances, master, nil
	}

	// get master node for the cluster
	if master, err = ou.orcClient.Master(ou.cluster.GetClusterAlias()); err != nil {
		if !orc.IsNotFound(err) {
			log.Error(err, "Orchestrator is not reachable", "cluster_alias", ou.cluster.GetClusterAlias())
			return instances, master, err
		}
		log.V(-1).Info("can't get master from Orchestrator", "msg", "not found", "alias", ou.cluster.GetClusterAlias())
	}

	// check if it's the same master with one that is determined from all instances
	insts := InstancesSet(instances)
	m := insts.DetermineMaster()
	if master == nil || m == nil || master.Key.Hostname != m.Key.Hostname {
		log.V(1).Info("master clash, between what is determined and what is in Orc", "fromOrc", instSummary(master), "determined", instSummary(m))
		return instances, nil, nil
	}

	log.V(1).Info("cluster master", "master", master.Key.Hostname, "cluster", ou.cluster.GetClusterAlias())
	return instances, master, nil
}

func (ou *orcUpdater) updateClusterReadyStatus() {
	if ou.cluster.Status.ReadyNodes != int(*ou.cluster.Spec.Replicas) {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReady,
			core.ConditionFalse, "StatefulSetNotReady", "StatefulSet is not ready")
		return
	}

	hasMaster := false
	for i := 0; i < int(*ou.cluster.Spec.Replicas); i++ {
		hostname := ou.cluster.GetPodHostname(i)
		ns := ou.cluster.GetNodeStatusFor(hostname)
		master := getCondAsBool(&ns, api.NodeConditionMaster)
		replicating := getCondAsBool(&ns, api.NodeConditionReplicating)

		if master {
			hasMaster = true
		} else if !replicating {
			ou.cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionFalse, "NotReplicating",
				fmt.Sprintf("Node %s is part of topology and not replicating", hostname))
			return
		}
	}

	if !hasMaster && !ou.cluster.Spec.ReadOnly && int(*ou.cluster.Spec.Replicas) > 0 {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReady, core.ConditionFalse, "NoMaster",
			"Cluster has no designated master")
		return
	}

	ou.cluster.UpdateStatusCondition(api.ClusterConditionReady,
		core.ConditionTrue, "ClusterReady", "Cluster is ready")
}

func getCondAsBool(status *api.NodeStatus, cond api.NodeConditionType) bool {
	index, exists := mysqlcluster.GetNodeConditionIndex(status, cond)
	return exists && status.Conditions[index].Status == core.ConditionTrue
}

// nolint: gocyclo
func (ou *orcUpdater) updateStatusFromOrc(insts InstancesSet, master *orc.Instance) {
	log.V(1).Info("updating nodes status", "insts", insts)

	// we assume that cluster is in ReadOnly
	isReadOnly := true

	// get maxSlaveLatency for this cluster
	maxSlaveLatency := defaultMaxSlaveLatency
	if ou.cluster.Spec.MaxSlaveLatency != nil {
		maxSlaveLatency = *ou.cluster.Spec.MaxSlaveLatency
	}

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

	// check if the master is up to date and is not downtime to remove in progress failover condition
	if master != nil && master.SecondsSinceLastSeen.Valid && master.SecondsSinceLastSeen.Int64 < 5 {
		log.Info("cluster failover finished", "master", master)
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverInProgress, core.ConditionFalse,
			"ClusterMasterHealthy", "Master is healthy in orchestrator")
	}
}

// updateNodesInOrc is the functions that tries to register
// unregistered nodes and to remove nodes that does not exists.
func (ou *orcUpdater) updateNodesInOrc(instances InstancesSet) (InstancesSet, []orc.InstanceKey, []orc.InstanceKey) {
	var (
		// hosts that should be discovered
		shouldDiscover []orc.InstanceKey
		// list of instances that should be removed from orchestrator
		shouldForget []orc.InstanceKey
		// list of instances from orchestrator that are present in k8s
		readyInstances InstancesSet
	)

	for i := 0; i < int(*ou.cluster.Spec.Replicas); i++ {
		host := ou.cluster.GetPodHostname(i)
		if inst := instances.GetInstance(host); inst == nil {
			// if index node is bigger than total ready nodes than should not be
			// added in discover list because maybe pod is not created yet
			if i < ou.cluster.Status.ReadyNodes {
				// host is not present into orchestrator
				// register new host into orchestrator
				hostKey := orc.InstanceKey{
					Hostname: host,
					Port:     mysqlPort,
				}
				shouldDiscover = append(shouldDiscover, hostKey)
			}
		} else {
			// this instance is present in both k8s and orchestrator
			readyInstances = append(readyInstances, *inst)
		}
	}

	// remove all instances from orchestrator that does not exists in k8s
	for _, inst := range instances {
		if i := readyInstances.GetInstance(inst.Key.Hostname); i == nil {
			shouldForget = append(shouldForget, inst.Key)
		}
	}

	if ou.cluster.DeletionTimestamp == nil {
		return readyInstances, shouldDiscover, shouldForget
	}

	toRemove := []orc.InstanceKey{}
	for _, i := range instances {
		toRemove = append(toRemove, i.Key)
	}
	return readyInstances, []orc.InstanceKey{}, toRemove
}

func (ou *orcUpdater) forgetNodesFromOrc(keys []orc.InstanceKey) {
	if len(keys) != 0 {
		log.Info("forget nodes in Orchestrator", "keys", keys)
	}
	// the only state in which a node can be removed from orchestrator
	// if cluster is ready or if cluster is deleted
	ready := ou.cluster.GetClusterCondition(api.ClusterConditionReady)
	if ready != nil && ready.Status == core.ConditionTrue &&
		time.Since(ready.LastTransitionTime.Time).Seconds() > forgetGraceTime ||
		ou.cluster.DeletionTimestamp != nil {
		// remove all instances from orchestrator that does not exists in k8s
		for _, key := range keys {
			if err := ou.orcClient.Forget(key.Hostname, key.Port); err != nil {
				log.Error(err, "failed to forget host with orchestrator", "key", key.Hostname)
			}
		}
	}
}

func (ou *orcUpdater) discoverNodesInOrc(keys []orc.InstanceKey) {
	if len(keys) != 0 {
		log.Info("discovering nodes in Orchestrator", "keys", keys)
	}
	for _, key := range keys {
		if err := ou.orcClient.Discover(key.Hostname, key.Port); err != nil {
			log.Error(err, "failed to discover host with orchestrator", "key", key)
		}
	}
}

func (ou *orcUpdater) getRecoveriesToAck(recoveries []orc.TopologyRecovery) []orc.TopologyRecovery {
	toAck := []orc.TopologyRecovery{}

	if len(recoveries) == 0 {
		return toAck
	}

	ready := ou.cluster.GetClusterCondition(api.ClusterConditionReady)
	if !(ready != nil && ready.Status == core.ConditionTrue &&
		time.Since(ready.LastTransitionTime.Time).Seconds() > recoveryGraceTime) {
		log.Info("cluster not ready for acknowledge", "threshold", recoveryGraceTime)
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
	comment := fmt.Sprintf("Statefulset '%s' is healthy for more than %d seconds",
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

// updateNodeCondition is a helper function that updates condition for a specific node
func (ou *orcUpdater) updateNodeCondition(host string, cType api.NodeConditionType, status core.ConditionStatus) {
	ou.cluster.UpdateNodeConditionStatus(host, cType, status)
}

// removeNodeConditionNotInOrc marks nodes not in orc with unknown condition
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

	// remove nodes status for nodes that are not desired, nodes that are left behind from scale down
	validIndex := 0
	for _, ns := range ou.cluster.Status.Nodes {
		// save only the nodes that are desired [0, 1, ..., replicas-1] or if index can't be extracted
		index, err := indexInSts(ns.Name)
		if err != nil {
			log.Info("failed to parse hostname for index - won't be removed", "error", err)
		}
		if index < *ou.cluster.Spec.Replicas || err != nil {
			ou.cluster.Status.Nodes[validIndex] = ns
			validIndex++
		}
	}

	// remove old nodes
	ou.cluster.Status.Nodes = ou.cluster.Status.Nodes[:validIndex]
}

// indexInSts is a helper function that returns the index of the pod in statefulset
func indexInSts(name string) (int32, error) {
	re := regexp.MustCompile(`^[\w-]+-mysql-(\d*)\.[\w-]*mysql(?:-nodes)?\.[\w-]+$`)
	values := re.FindStringSubmatch(name)
	if len(values) != 2 {
		return 0, fmt.Errorf("no match found")
	}

	i, err := strconv.Atoi(values[1])
	return int32(i), err
}

// set a host writable just if needed
func (ou *orcUpdater) setWritableNode(inst orc.Instance) error {
	if inst.ReadOnly {
		log.Info("set node writable", "node", inst.Key.Hostname)
		return ou.orcClient.SetHostWritable(inst.Key)
	}
	return nil
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
func (ou *orcUpdater) markReadOnlyNodesInOrc(insts InstancesSet, master *orc.Instance) {
	var err error
	if master == nil {
		// master is not found
		// set cluster read only
		log.Info("setting cluster in read-only", "cluster", ou.cluster.GetClusterAlias())
		for _, inst := range insts {
			if err = ou.setReadOnlyNode(inst); err != nil {
				log.Error(err, "failed to set read only", "instance", inst)
			}
		}
		return
	}

	// master is determined
	for _, inst := range insts {
		if ou.cluster.Spec.ReadOnly {
			if err = ou.setReadOnlyNode(inst); err != nil {
				log.Error(err, "failed to set read only", "instance", inst)
			}
		} else {
			// set master writable or replica read-only
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
	if len(node.MasterKey.Hostname) != 0 && !node.IsCoMaster && !node.IsDetachedMaster {
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
			log.V(1).Info("DetermineMaster: master not found for node", "node", node.Key.Hostname)
			return nil
		}
		masterForNode = append(masterForNode, *master)
	}

	if len(masterForNode) != 0 {
		masterHostName := masterForNode[0]
		for _, node := range masterForNode {
			if node.Key.Hostname != masterHostName.Key.Hostname {
				log.V(1).Info("DetermineMaster: a node has different master", "node", node.Key.Hostname,
					"master", masterForNode)
				return nil
			}
		}
		return &masterHostName
	}

	log.V(1).Info("DetermineMaster: master not set", "instances", is)
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

func instSummary(inst *orc.Instance) string {
	if inst == nil {
		return "nil"
	}

	masterInfo := fmt.Sprintf(",master=%s:%d", inst.MasterKey.Hostname, inst.MasterKey.Port)

	return fmt.Sprintf("key=%s:%d,%s", inst.Key.Hostname, inst.Key.Port,
		masterInfo)
}
