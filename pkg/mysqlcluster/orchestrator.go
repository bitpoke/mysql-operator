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

	"github.com/golang/glog"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

type ClusterInfo struct {
	api.MysqlCluster

	// Master represent the cluster master hostname
	MasterHostname string
}

var SavedClusters map[string]ClusterInfo

func (f *cFactory) registerNodesInOrc() error {
	// Register nodes in orchestrator
	if len(f.cluster.Spec.GetOrcUri()) != 0 {
		// try to discover ready nodes into orchestrator
		client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
		for i := 0; i < int(f.cluster.Status.ReadyNodes); i++ {
			host := f.getHostForReplica(i)
			if err := client.Discover(host, MysqlPort); err != nil {
				glog.Warningf("Failed to register %s with orchestrator: %s", host, err.Error())
			}
		}
	}

	return nil
}

func (f *cFactory) updateClusterMasterFromOrc() error {
	masterHost := f.cluster.GetPodHostName(0)

	if len(f.cluster.Spec.GetOrcUri()) != 0 {
		// try to discover ready nodes into orchestrator
		client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
		orcClusterName := fmt.Sprintf("%s.%s", f.cluster.Name, f.cluster.Namespace)
		if inst, err := client.Master(orcClusterName); err == nil {
			masterHost = inst.Key.Hostname
		} else {
			glog.Warningf(
				"Failed getting master for %s: %s, falling back to default.",
				orcClusterName, err,
			)
		}
	}

	if len(SavedClusters) == 0 {
		SavedClusters = make(map[string]ClusterInfo)
	}

	SavedClusters[f.cluster.Name] = ClusterInfo{
		MysqlCluster:   *f.cluster.DeepCopy(),
		MasterHostname: masterHost,
	}

	return nil
}
