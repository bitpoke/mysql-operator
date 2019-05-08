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
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

// newFinalizerSyncer returns a syncer for mysql cluster that sets the OrchestratorFinalizer
func newFinalizerSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, orcClient orc.Interface) syncer.Interface {

	// create the orchestrator finalizer
	return syncer.NewObjectSyncer("OrchestratorFinalizerSyncer", nil, cluster.Unwrap(), c, scheme, func(in runtime.Object) error {
		out := in.(*api.MysqlCluster)

		// always add finalizer, this action is idempotent
		addFinalizer(out, OrchestratorFinalizer)
		// TODO: remove this in next version (v0.4)
		removeFinalizer(out, OldOrchestratorFinalizer)

		// if cluster is deleted then check orchestrator status and remove finalizer if no node is in orchestrator
		if out.DeletionTimestamp != nil {
			var (
				instances InstancesSet
				err       error
			)
			// get status from orchestrator
			if instances, err = orcClient.Cluster(cluster.GetClusterAlias()); err != nil {
				log.V(-1).Info("an error occurred while getting cluster from orchestrator", "error", err)
			}

			if len(instances) == 0 {
				removeFinalizer(out, OrchestratorFinalizer)
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
			in.Finalizers = append(in.Finalizers[:index], in.Finalizers[index+1:]...)
		}
	}
}
