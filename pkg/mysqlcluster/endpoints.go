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
	"time"

	core "k8s.io/api/core/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const (
	allowedLagCycles = 3.0
)

func (f *cFactory) updateMasterServiceEndpoints() error {
	masterHost := f.getMasterHost()
	if err := f.updatePodLabels(masterHost); err != nil {
		return err
	}

	return f.addNodesToService(f.cluster.GetNameForResource(api.MasterService), masterHost)
}

func (f *cFactory) updateHealthyNodesServiceEndpoints() error {
	var nodes []string
	for _, ns := range f.cluster.Status.Nodes {
		master := ns.GetCondition(api.NodeConditionMaster)
		replicating := ns.GetCondition(api.NodeConditionReplicating)
		lagged := ns.GetCondition(api.NodeConditionLagged)
		if master == nil || replicating == nil || lagged == nil {
			continue
		}

		isLagged := false
		if f.cluster.Spec.MaxSlaveLatency != nil {
			isLagged = lagged.Status == core.ConditionTrue &&
				time.Since(lagged.LastTransitionTime.Time).Seconds() >
					float64(*f.cluster.Spec.MaxSlaveLatency)*allowedLagCycles
		}

		if master.Status == core.ConditionTrue || master.Status == core.ConditionFalse &&
			replicating.Status == core.ConditionTrue && !isLagged {
			nodes = append(nodes, ns.Name)
		}
	}

	if len(nodes) > 0 {
		return f.addNodesToService(f.cluster.GetNameForResource(api.HealthyNodesService),
			nodes...)
	}

	return nil
}
