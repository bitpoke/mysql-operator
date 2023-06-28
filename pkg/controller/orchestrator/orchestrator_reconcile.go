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

	"github.com/go-logr/logr"
	logf "github.com/presslabs/controller-util/log"
	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/controller/node"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/bitpoke/mysql-operator/pkg/orchestrator"
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
	uptimeGraceTime              = 15
)

type orcUpdater struct {
	cluster   *mysqlcluster.MysqlCluster
	client    client.Client
	recorder  record.EventRecorder
	orcClient orc.Interface

	log logr.Logger
}

// NewOrcUpdater returns a syncer that updates cluster status from orchestrator.
func NewOrcUpdater(cluster *mysqlcluster.MysqlCluster, r record.EventRecorder, orcClient orc.Interface, client client.Client) syncer.Interface {
	return &orcUpdater{
		client:    client,
		cluster:   cluster,
		recorder:  r,
		orcClient: orcClient,
		log:       logf.Log.WithName("orchestrator-reconciler").WithValues("key", cluster.GetNamespacedName()),
	}
}

func (ou *orcUpdater) Object() interface{}         { return nil }
func (ou *orcUpdater) ObjectOwner() runtime.Object { return ou.cluster }
func (ou *orcUpdater) GetObject() interface{}      { return nil }
func (ou *orcUpdater) GetOwner() runtime.Object    { return ou.cluster }
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

	// get recoveries for this cluster
	if recoveries, err = ou.orcClient.AuditRecovery(ou.cluster.GetClusterAlias()); err != nil {
		ou.log.V(0).Info("can't get recoveries from orchestrator", "error", err.Error())
	}

	// update cluster status
	ou.updateNodesStatus(instances, master)
	ou.updateClusterFailoverInProgressStatus(master)
	ou.updateClusterReadOnlyStatus(instances)
	ou.updateClusterReadyStatus()
	ou.updateFailoverAckStatus(recoveries)

	// remove old nodes from orchestrator, depends on cluster ready status
	ou.forgetNodesFromOrc(toRemoveInstances)

	// filter recoveries that can be acknowledged
	toAck := ou.getRecoveriesToAck(recoveries)

	// acknowledge recoveries, depends on cluster ready status
	if err = ou.acknowledgeRecoveries(toAck); err != nil {
		ou.log.Error(err, "failed to acknowledge recoveries", "ack_recoveries", toAck)
	}

	return syncer.SyncResult{}, nil
}

func (ou *orcUpdater) getFromOrchestrator() (instances []orc.Instance, master *orc.Instance, err error) {

	// get all related instances from orchestrator
	if instances, err = ou.orcClient.Cluster(ou.cluster.GetClusterAlias()); err != nil {
		if !orc.IsNotFound(err) {
			ou.log.Error(err, "Orchestrator is not reachable")
			return instances, master, err
		}
		ou.log.V(0).Info("cluster not found in Orchestrator", "error", "not found")
		return instances, master, nil
	}

	// get master node for the cluster
	if master, err = ou.orcClient.Master(ou.cluster.GetClusterAlias()); err != nil {
		if !orc.IsNotFound(err) {
			ou.log.Error(err, "Orchestrator is not reachable")
			return instances, master, err
		}
		ou.log.V(0).Info("can't get master from Orchestrator", "error", "not found")
	}

	// check if it's the same master with one that is determined from all instances
	insts := InstancesSet(instances)
	m := insts.DetermineMaster()
	if master == nil || m == nil || master.Key.Hostname != m.Key.Hostname {
		// throw a warning
		ou.log.V(0).Info("master clash, between what is determined and what is in Orc",
			"in_orchestrator", instToLog(master), "determined", instToLog(m))
		return instances, nil, nil
	}

	ou.log.V(1).Info("cluster master", "master", master.Key.Hostname)
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
			// TODO: check for replicating to be not Unknown here
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
func (ou *orcUpdater) updateNodesStatus(insts InstancesSet, master *orc.Instance) {
	ou.log.V(1).Info("updating nodes status", "instances", insts.ToLog())

	// get maxSlaveLatency for this cluster
	maxSlaveLatency := defaultMaxSlaveLatency
	if ou.cluster.Spec.MaxSlaveLatency != nil {
		maxSlaveLatency = *ou.cluster.Spec.MaxSlaveLatency
	}

	// update conditions for every node
	for _, node := range insts {
		host := node.Key.Hostname

		// nodes that are not up to date in orchestrator should be marked as unknown
		if !node.IsRecentlyChecked {
			log.V(1).Info("Orchestrator detected host as stale", "host", host)

			if !node.IsLastCheckValid {
				log.V(1).Info("Last orchestrator host check invalid", "host", host)
				ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionUnknown)
				ou.updateNodeCondition(host, api.NodeConditionReplicating, core.ConditionUnknown)
				ou.updateNodeCondition(host, api.NodeConditionMaster, core.ConditionUnknown)
			}
			continue
		}

		// set node Lagged conditions
		if master != nil && host == master.Key.Hostname {
			// sometimes the pt-hearbeat is slowed down, but it doesn't mean the master
			// is lagging (it's not replicating). So always set False for master.
			ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionFalse)
		} else {
			if !node.SlaveLagSeconds.Valid {
				ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionUnknown)
			} else if node.SlaveLagSeconds.Int64 <= maxSlaveLatency {
				ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionFalse)
			} else { // node is behind master
				ou.updateNodeCondition(host, api.NodeConditionLagged, core.ConditionTrue)
			}
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
	}
}

func (ou *orcUpdater) updateClusterReadOnlyStatus(insts InstancesSet) {
	var readOnlyHosts []string
	var writableHosts []string

	for _, node := range insts {
		host := node.Key.Hostname
		// set node read only
		if node.ReadOnly {
			readOnlyHosts = append(readOnlyHosts, host)
		} else {
			writableHosts = append(writableHosts, host)
		}
	}

	// set cluster ReadOnly condition
	if len(writableHosts) == 0 {
		// cluster is read-only
		msg := fmt.Sprintf("read-only nodes: %s", strings.Join(readOnlyHosts, " "))
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionTrue, "ClusterReadOnlyTrue", msg)
	} else {
		// cluster is writable
		msg := fmt.Sprintf("writable nodes: %s", strings.Join(writableHosts, " "))
		ou.cluster.UpdateStatusCondition(api.ClusterConditionReadOnly,
			core.ConditionFalse, "ClusterReadOnlyFalse", msg)
	}
}

func (ou *orcUpdater) updateClusterFailoverInProgressStatus(master *orc.Instance) {
	// check if the master is up to date and is not downtime to remove in progress failover condition
	if master != nil && master.SecondsSinceLastSeen.Valid && master.SecondsSinceLastSeen.Int64 < 5 {
		ou.cluster.UpdateStatusCondition(api.ClusterConditionFailoverInProgress, core.ConditionFalse,
			"ClusterMasterHealthy", "Master is healthy in orchestrator")
	}
}

// updateNodesInOrc is the functions that tries to register
// unregistered nodes and to remove nodes that does not exists.
// nolint:gocyclo
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
				go func(i int) {
					if ou.client == nil {
						return
					}
					// check if the pod is running
					// if pod is running, we should gen a event to let the replica reconnect to the master
					pod := &core.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("%s-mysql-%d", ou.cluster.Name, i),
							Namespace: ou.cluster.Namespace,
						},
					}
					err := ou.client.Get(context.Background(), client.ObjectKeyFromObject(pod), pod)
					if err != nil {
						return
					}
					if pod.Status.Phase == core.PodRunning {
						node.NodeGenericEvents <- event.GenericEvent{
							Object: pod,
						}
					}
				}(i)
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
		ou.log.Info("forget nodes in Orchestrator", "instances", keys)
	}
	// the only allowed state in which a node can be removed from orchestrator is
	// weather the cluster is ready or if it's deleted
	ready := ou.cluster.GetClusterCondition(api.ClusterConditionReady)
	if ready != nil && ready.Status == core.ConditionTrue &&
		time.Since(ready.LastTransitionTime.Time).Seconds() > forgetGraceTime ||
		ou.cluster.DeletionTimestamp != nil {
		// remove all instances from orchestrator that does not exists in k8s
		for _, key := range keys {
			if err := ou.orcClient.Forget(key.Hostname, key.Port); err != nil {
				ou.log.Error(err, "failed to forget host with orchestrator", "instance", key.Hostname)
			}
		}
	}
}

func (ou *orcUpdater) discoverNodesInOrc(keys []orc.InstanceKey) {
	if len(keys) != 0 {
		ou.log.Info("discovering nodes in Orchestrator", "instances", keys)
	}
	for _, key := range keys {
		if err := ou.orcClient.Discover(key.Hostname, key.Port); err != nil {
			ou.log.Error(err, "failed to discover host with orchestrator", "instance", key)
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
		ou.log.Info("cluster not ready for acknowledge", "threshold", recoveryGraceTime)
		return toAck
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then <recoveryGraceTime> seconds
			recoveryStartTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				ou.log.Error(err, "time parse error", "recovery", recovery)
				continue
			}
			if time.Since(recoveryStartTime).Seconds() < recoveryGraceTime {
				// skip this recovery
				ou.log.V(1).Info("tries to recover to soon", "recovery", recovery)
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

func (ou *orcUpdater) updateFailoverAckStatus(recoveries []orc.TopologyRecovery) {
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
			ou.log.Info("failed to parse hostname for index - won't be removed", "error", err)
		}

		if !shouldRemoveOldNode(&ns, ou.cluster) && index < *ou.cluster.Spec.Replicas || err != nil {
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
		ou.log.Info("set node writable", "instance", instToLog(&inst))
		return ou.orcClient.SetHostWritable(inst.Key)
	}
	return nil
}

// set a host read only just if needed
func (ou *orcUpdater) setReadOnlyNode(inst orc.Instance) error {
	if !inst.ReadOnly {
		ou.log.Info("set node read only", "instance", instToLog(&inst))
		return ou.orcClient.SetHostReadOnly(inst.Key)
	}
	return nil
}

// nolint: gocyclo
func (ou *orcUpdater) markReadOnlyNodesInOrc(insts InstancesSet, master *orc.Instance) {
	// If there is an in-progress failover, we will not interfere with readable/writable status on this iteration.
	fip := ou.cluster.GetClusterCondition(api.ClusterConditionFailoverInProgress)
	if fip != nil && fip.Status == core.ConditionTrue {
		ou.log.Info("cluster has a failover in progress, will delay setting readable/writeable status until failover is complete",
			"since", fip.LastTransitionTime)
		return
	}
	var err error
	if master == nil {
		// master is not found
		// set cluster read only
		for _, inst := range insts {
			// give time to stabilize in case of a failover
			if !inst.IsUpToDate || inst.Uptime < uptimeGraceTime {
				ou.log.Info("skip set read-only/writable", "instance", instToLog(&inst))
				continue
			}
			if err = ou.setReadOnlyNode(inst); err != nil {
				ou.log.Error(err, "failed to set read only", "instance", instToLog(&inst))
			}
		}
		return
	}

	// master is determined
	for _, inst := range insts {
		// give time to stabilize in case of a failover
		// https://github.com/bitpoke/mysql-operator/issues/566
		if !inst.IsUpToDate || inst.Uptime < uptimeGraceTime {
			ou.log.Info("skip set read-only/writable", "instance", instToLog(&inst))
			continue
		}

		if ou.cluster.Spec.ReadOnly {
			if err = ou.setReadOnlyNode(inst); err != nil {
				ou.log.Error(err, "failed to set read only", "instance", instToLog(&inst))
			}
		} else {
			// set master writable or replica read-only
			if inst.Key.Hostname == master.Key.Hostname {
				if err = ou.setWritableNode(inst); err != nil {
					ou.log.Error(err, "failed to set writable", "instance", instToLog(&inst))
				}
			} else {
				if err = ou.setReadOnlyNode(inst); err != nil {
					log.Error(err, "failed to set read only", "instance", instToLog(&inst))
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

func (is InstancesSet) getMasterForNode(node *orc.Instance, visited []*orc.Instance) *orc.Instance {
	// Check for infinite loops. We don't want to follow a node that was already visited.
	// This may happen when node are not marked as CoMaster by orchestrator
	for _, vn := range visited {
		if vn.Key == node.Key {
			return nil
		}
	}
	visited = append(visited, node)

	if len(node.MasterKey.Hostname) != 0 && !node.IsCoMaster && !node.IsDetachedMaster {
		// get the (maybe intermediate) master hostname from MasterKey if MasterKey is set
		master := is.GetInstance(node.MasterKey.Hostname)
		if master != nil {
			return is.getMasterForNode(master, visited)
		}
		return nil
	}

	if node.IsCoMaster {
		// if it's CoMaster then return the other master
		master := is.GetInstance(node.MasterKey.Hostname)
		return master
	}

	return node
}

// DetermineMaster returns a orc.Instance that is master or nil if can't find master
func (is InstancesSet) DetermineMaster() *orc.Instance {
	masterForNode := []orc.Instance{}

	for _, node := range is {
		master := is.getMasterForNode(&node, []*orc.Instance{})
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

// ToLog returns a list of simplified output for logging
func (is InstancesSet) ToLog() []map[string]string {
	output := []map[string]string{}
	for _, inst := range is {
		output = append(output, instToLog(&inst))
	}
	return output
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

func instToLog(inst *orc.Instance) map[string]string {
	if inst == nil {
		return nil
	}

	return map[string]string{
		"Hostname":       inst.Key.Hostname,
		"MasterHostname": inst.MasterKey.Hostname,
		"IsUpToDate":     strconv.FormatBool(inst.IsUpToDate),
	}
}

func shouldRemoveOldNode(node *api.NodeStatus, cluster *mysqlcluster.MysqlCluster) bool {
	if version, ok := cluster.ObjectMeta.Annotations["mysql.presslabs.org/version"]; ok && version == "300" {
		return strings.Contains(node.Name, "-mysql-nodes")
	}

	return false
}
