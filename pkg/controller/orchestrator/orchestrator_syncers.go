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
	"github.com/presslabs/controller-util/syncer"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

// newFinalizerSyncer returns a syncer for mysql cluster that sets the OrchestratorFinalizer
func newFinalizerSyncer(c client.Client, scheme *runtime.Scheme, cluster *api.MysqlCluster, orcClient orc.Interface) syncer.Interface {

	// create the orchestrator finalizer
	return syncer.NewObjectSyncer("OrchestratorFinalizerSyncer", nil, cluster, c, scheme, func(in runtime.Object) error {
		cluster := in.(*api.MysqlCluster)

		// always add finalizer, this action is idempotent
		addFinalizer(cluster, OrchestratorFinalizer)

		// if cluster is deleted then check orchestrator status and remove finalizer if no node is in orchestrator
		if cluster.DeletionTimestamp != nil {
			var (
				instances InstancesSet
				err       error
			)
			// get status from orchestrator
			if instances, err = orcClient.Cluster(wrapcluster.NewMysqlClusterWrapper(cluster).GetClusterAlias()); err != nil {
				log.Error(err, "can't get instances from orchestrator", "cluster", cluster)
			}

			if len(instances) == 0 {
				removeFinalizer(cluster, OrchestratorFinalizer)
			}
		}

		return nil
	})
}

func addFinalizer(in *api.MysqlCluster, finalizer string) {
	for _, f := range in.Finalizers {
		if f == finalizer {
			// finalizer already exists
			return
		}
	}

	// initialize list
	if len(in.Finalizers) == 0 {
		in.Finalizers = []string{}
	}

	in.Finalizers = append(in.Finalizers, finalizer)
}

func removeFinalizer(in *api.MysqlCluster, finalizer string) {
	var (
		index int
		f     string
	)
	for index, f = range in.Finalizers {
		if f == finalizer {
			break
		}
	}

	in.Finalizers = append(in.Finalizers[:index], in.Finalizers[index+1:]...)
}
