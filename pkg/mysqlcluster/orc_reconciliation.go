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

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

const (
	allowedNodeLagSeconds = 30
)

// ReconcileORC function is called in a loop and should update cluster status
// with latest information from orchestrator or to register the new nodes into
// orchestrator.
func (f *cFactory) ReconcileORC(ctx context.Context) error {
	glog.Infof("Orchestrator reconciliation for cluster %s started...", f.cluster.Name)
	if len(f.cluster.Spec.GetOrcUri()) == 0 {
		return fmt.Errorf("orchestrator is not configured")
	}

	client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
	if insts, err := client.Cluster(f.cluster.Name); err != nil {
		f.updateStatusFromOrc(insts)
	} else {
		return f.registerNodesInOrc()
	}

	return nil
}

func (f *cFactory) updateStatusFromOrc(insts []orc.Instance) {
	for i := 0; i < int(f.cluster.Spec.Replicas); i++ {
		host := f.getHostForReplica(i)
		// select instance from orchestrator
		var node *orc.Instance
		for _, inst := range insts {
			if inst.Key.Hostname == host {
				node = &inst
				break
			}
		}
		status := f.cluster.Status.GetNodeStatus(host)

		if node != nil {
			status.UpdateNodeCondition(api.NodeConditionLagged, core.ConditionUnknown)
			status.UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionUnknown)
			status.UpdateNodeCondition(api.NodeConditionMaster, core.ConditionUnknown)

			return
		}

		if node.SecondsBehindMaster.Valid && node.SecondsBehindMaster.Int64 <= allowedNodeLagSeconds {
			status.UpdateNodeCondition(api.NodeConditionLagged, core.ConditionFalse)
		} else { // node is behind master
			status.UpdateNodeCondition(api.NodeConditionLagged, core.ConditionTrue)
		}

		if node.Slave_SQL_Running && node.Slave_IO_Running {
			status.UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionTrue)
		} else {
			status.UpdateNodeCondition(api.NodeConditionReplicating, core.ConditionFalse)
		}

		if !node.ReadOnly {
			status.UpdateNodeCondition(api.NodeConditionMaster, core.ConditionTrue)
		} else {
			status.UpdateNodeCondition(api.NodeConditionMaster, core.ConditionFalse)
		}

	}
}

func (f *cFactory) registerNodesInOrc() error {
	// Register nodes in orchestrator
	// try to discover ready nodes into orchestrator
	client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
	for i := 0; i < int(f.cluster.Status.ReadyNodes); i++ {
		host := f.getHostForReplica(i)
		if err := client.Discover(host, MysqlPort); err != nil {
			glog.Warningf("Failed to register %s with orchestrator: %s", host, err.Error())
		}
	}

	return nil
}
