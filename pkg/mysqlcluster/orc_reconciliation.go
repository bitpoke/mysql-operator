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
	"fmt"
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

const (
	healtyMoreThanMinutes = 10
)

// ReconcileORC function is called in a loop and should update cluster status
// with latest information from orchestrator or to register the new nodes into
// orchestrator.
func (f *cFactory) ReconcileORC(ctx context.Context) error {
	glog.Infof("Orchestrator reconciliation for cluster %s started...", f.cluster.Name)
	if f.orcClient == nil {
		return fmt.Errorf("orchestrator is not configured")
	}

	if insts, err := f.orcClient.Cluster(f.getClusterAlias()); err == nil {
		f.updateStatusFromOrc(insts)
	} else {
		glog.Errorf("Fail to get cluster from orchestrator: %s. Now tries to register nodes.", err)
		return f.registerNodesInOrc()
	}

	if recoveries, err := f.orcClient.AuditRecovery(f.getClusterAlias()); err == nil {
		f.updateStatusForRecoveries(recoveries)
		toAck := f.getRecoveriesToAck(recoveries)

		comment := fmt.Sprintf("Statefulset '%s' is healty more then 10 minutes",
			f.cluster.GetNameForResource(api.StatefulSet),
		)

		// acknowledge recoveries
		for _, r := range toAck {
			if err := f.orcClient.AckRecovery(r.Id, comment); err != nil {
				glog.Errorf("Trying to ack recovery with id %d but failed with error: %s",
					r.Id, err,
				)
			}
		}
	}

	return nil
}

func (f *cFactory) updateStatusFromOrc(insts []orc.Instance) {
	for i := 0; i < int(f.cluster.Spec.Replicas); i++ {
		host := f.cluster.GetPodHostName(i)
		// select instance from orchestrator
		var node *orc.Instance
		for _, inst := range insts {
			if inst.Key.Hostname == host {
				node = &inst
				break
			}
		}
		i := f.cluster.Status.GetNodeStatusIndex(host)

		if node == nil {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionLagged, core.ConditionUnknown)
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionUnknown)
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionMaster, core.ConditionUnknown)

			return
		}

		if !node.SlaveLagSeconds.Valid {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionLagged, core.ConditionUnknown)
		} else if node.SlaveLagSeconds.Int64 <= *f.cluster.Spec.MaxSlaveLatency {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionLagged, core.ConditionFalse)
		} else { // node is behind master
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionLagged, core.ConditionTrue)
		}

		if node.Slave_SQL_Running && node.Slave_IO_Running {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionTrue)
		} else {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionFalse)
		}

		if !node.ReadOnly {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionMaster, core.ConditionTrue)
		} else {
			f.cluster.Status.Nodes[i].UpdateNodeCondition(api.NodeConditionMaster, core.ConditionFalse)
		}
	}
}

func (f *cFactory) updateStatusForRecoveries(recoveries []orc.TopologyRecovery) {
	var unack []orc.TopologyRecovery
	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			unack = append(unack, recovery)
		}
	}

	if len(unack) > 0 {
		f.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionTrue, "pendingFailoverAckExists", fmt.Sprintf("%#v", unack))
	} else {
		f.cluster.UpdateStatusCondition(api.ClusterConditionFailoverAck,
			core.ConditionFalse, "noPendingFailoverAckExists", "no pending ack")
	}
}

func (f *cFactory) registerNodesInOrc() error {
	// Register nodes in orchestrator
	// try to discover ready nodes into orchestrator
	for i := 0; i < int(f.cluster.Status.ReadyNodes); i++ {
		host := f.cluster.GetPodHostName(i)
		if err := f.orcClient.Discover(host, MysqlPort); err != nil {
			glog.Warningf("Failed to register %s with orchestrator: %s", host, err.Error())
		}
	}

	return nil
}

func (f *cFactory) getRecoveriesToAck(recoveries []orc.TopologyRecovery) (toAck []orc.TopologyRecovery) {
	if len(recoveries) == 0 {
		return
	}

	i, find := condIndexCluster(f.cluster, api.ClusterConditionReady)
	if !find || f.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		glog.Warning("[getRecoveriesToAck]: Cluster is not ready for ack.")
		return
	}

	if time.Since(f.cluster.Status.Conditions[i].LastTransitionTime.Time).Minutes() < healtyMoreThanMinutes {
		glog.Warning(
			"[getRecoveriesToAck]: Stateful set is not ready more then 10 minutes. Don't ack.",
		)
		return
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then <healtyMoreThanMinutes> minutes
			startTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				glog.Errorf("[getRecoveriesToAck] Can't parse time: %s for audit recovery: %d",
					err, recovery.Id,
				)
				continue
			}
			if time.Since(startTime).Minutes() < healtyMoreThanMinutes {
				// skip this recovery
				glog.Errorf("[getRecoveriesToAck] recovery to soon")
				continue
			}

			toAck = append(toAck, recovery)
		}
	}
	return
}

func condIndexCluster(r *api.MysqlCluster, ty api.ClusterConditionType) (int, bool) {
	for i, cond := range r.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}
