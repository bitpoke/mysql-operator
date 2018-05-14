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

func (f *cFactory) updateMasterServiceEndpoints() error {
	masterHost := f.cluster.GetPodHostName(0)

	for _, ns := range f.cluster.Status.Nodes {
		if cond := ns.GetCondition(api.NodeConditionMaster); cond != nil {
			masterHost = ns.Name
		}
	}

	return f.addNodesToService(f.cluster.GetNameForResource(api.MasterService), masterHost)
}

func (f *cFactory) autoAcknowledge() error {
	// TODO: does not work when cluster does not change more than 10 minutes after failover
	i, find := condIndexCluster(f.cluster, api.ClusterConditionFailoverAck)
	if !find || f.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		glog.V(3).Infof("[autoAcknowledge]: Nothing to do for cluuster %s", f.cluster.Name)
		return nil
	}

	i, find = condIndexCluster(f.cluster, api.ClusterConditionReady)
	if !find || f.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		glog.Warning("[autoAcknowledge]: Cluster is not ready for ack.")
		return nil
	}

	if time.Since(f.cluster.Status.Conditions[i].LastTransitionTime.Time).Minutes() < healtyMoreThanMinutes {
		glog.Warning(
			"[autoAcknowledge]: Stateful set is not ready more then 10 minutes. Don't ack.",
		)
		return nil
	}

	// proceed with cluster recovery
	client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
	recoveries, err := client.AuditRecovery(f.getClusterAlias())
	if err != nil {
		return fmt.Errorf("orchestrator audit: %s", err)
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then <healtyMoreThanMinutes> minutes
			startTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				glog.Errorf("[autoAcknowledge] Can't parse time: %s for audit recovery: %d",
					err, recovery.Id,
				)
				continue
			}
			if time.Since(startTime).Minutes() < healtyMoreThanMinutes {
				// skip this recovery
				continue
			}

			comment := fmt.Sprintf("Statefulset '%s' is healty more then 10 minutes",
				f.cluster.GetNameForResource(api.StatefulSet),
			)
			if err := client.AckRecovery(recovery.Id, comment); err != nil {
				glog.Errorf("Trying to ack recovery with id %d but failed with error: %s",
					recovery.Id, err,
				)
			}
		}
	}

	return nil
}

func condIndexCluster(r *api.MysqlCluster, ty api.ClusterConditionType) (int, bool) {
	for i, cond := range r.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}
