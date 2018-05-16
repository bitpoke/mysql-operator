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
	masterHost := f.cluster.GetPodHostName(0)

	for _, ns := range f.cluster.Status.Nodes {
		if cond := ns.GetCondition(api.NodeConditionMaster); cond != nil &&
			cond.Status == core.ConditionTrue {
			masterHost = ns.Name
		}
	}

	return f.addNodesToService(f.cluster.GetNameForResource(api.MasterService), masterHost)
}

func (f *cFactory) updateHealtyNodesServiceEndpoints() error {
	var hNodes []string
	for i := 0; i < len(f.cluster.Status.Nodes); i++ {
		ns := f.cluster.Status.Nodes[i]

		master := ns.GetCondition(api.NodeConditionMaster)
		replicating := ns.GetCondition(api.NodeConditionReplicating)
		lagged := ns.GetCondition(api.NodeConditionLagged)
		if master == nil || replicating == nil || lagged == nil {
			continue
		}

		isLagged := lagged.Status == core.ConditionTrue &&
			time.Since(lagged.LastTransitionTime.Time).Seconds() >
				float64(*f.cluster.Spec.MaxSlaveLatency)*allowedLagCycles

		if master.Status == core.ConditionTrue || master.Status == core.ConditionFalse &&
			replicating.Status == core.ConditionTrue && !isLagged {
			hNodes = append(hNodes, ns.Name)
		}
	}

	if len(hNodes) > 0 {
		return f.addNodesToService(f.cluster.GetNameForResource(api.HealtyNodesService),
			hNodes...)
	}

	return nil
}
