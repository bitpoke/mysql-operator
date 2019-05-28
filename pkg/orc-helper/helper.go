/*
Copyright 2019 Pressinfra SRL

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

package orchelper

import (
	"context"
	"fmt"
	"strings"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// parse the orchestrator cluster name as NamespacedName
func orcNameToKey(name string) (types.NamespacedName, error) {
	components := strings.Split(name, ".")
	if len(components) != 2 {
		return types.NamespacedName{}, fmt.Errorf("can't parse name: %s", name)
	}

	return types.NamespacedName{Name: components[0], Namespace: components[1]}, nil
}

// UpdateClusterFailoverCond updates the cluster FailoverInProgress condition on the given cluster
func UpdateClusterFailoverCond(c client.Client, clusterName, reason, msg string, status bool) error {
	key, err := orcNameToKey(clusterName)
	if err != nil {
		return err
	}

	cluster := mysqlcluster.New(&api.MysqlCluster{})

	// get cluster from k8s
	if err := c.Get(context.TODO(), key, cluster.Unwrap()); err != nil {
		return err
	}

	s := core.ConditionFalse
	if status {
		s = core.ConditionTrue
	}

	// update cluster failover in progress condition
	cluster.UpdateStatusCondition(api.ClusterConditionFailoverInProgress, s, reason, msg)

	// update cluster status
	if err := c.Status().Update(context.TODO(), cluster.Unwrap()); err != nil {
		return err
	}

	return nil
}
