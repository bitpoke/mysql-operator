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

package mysqlcluster

import (
	"github.com/presslabs/controller-util/rand"
	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	rStrLen = 18
)

// NewOperatedSecretSyncer returns secret syncer
// nolint: gocyclo
func NewOperatedSecretSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, opt *options.Options) syncer.Interface {
	obj := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.Secret),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("OperatedSecret", nil, obj, c, scheme, func(in runtime.Object) error {
		out := in.(*core.Secret)

		// the user used for operator to connect to the mysql node for configuration
		out.Data["OPERATOR_USER"] = []byte("sys_operator")
		if len(out.Data["REPLICATION_PASSWORD"]) == 0 {
			random, err := rand.ASCIIString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["OPERATOR_PASSWORD"] = []byte(random)
		}

		// the user that is used to configure replication between nodes
		out.Data["REPLICATION_USER"] = []byte("sys_replication")
		if len(out.Data["REPLICATION_PASSWORD"]) == 0 {
			random, err := rand.ASCIIString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["REPLICATION_PASSWORD"] = []byte(random)
		}

		// the user that is used by the metrics exporter sidecar to collect mysql metrics
		out.Data["METRICS_EXPORTER_USER"] = []byte("sys_exporter")
		if len(out.Data["METRICS_EXPORTER_PASSWORD"]) == 0 {
			random, err := rand.AlphaNumericString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["METRICS_EXPORTER_PASSWORD"] = []byte(random)
		}

		// the user that is used by orchestrator to manage topology and failover
		out.Data["ORC_TOPOLOGY_USER"] = []byte(opt.OrchestratorTopologyUser)
		out.Data["ORC_TOPOLOGY_PASSWORD"] = []byte(opt.OrchestratorTopologyPassword)

		// the user that is used to serve backups over HTTP
		out.Data["BACKUP_USER"] = []byte("sys_backups")
		if len(out.Data["BACKUP_PASSWORD"]) == 0 {
			random, err := rand.ASCIIString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["BACKUP_PASSWORD"] = []byte(random)
		}

		return nil
	})
}
