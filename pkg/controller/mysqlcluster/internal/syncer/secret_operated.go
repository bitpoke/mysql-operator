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
func NewOperatedSecretSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, opt *options.Options) syncer.Interface {
	obj := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.Secret),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("OperatedSecret", cluster.Unwrap(), obj, c, scheme, func(in runtime.Object) error {
		out := in.(*core.Secret)

		if out.Data == nil {
			out.Data = make(map[string][]byte)
		}

		// the user used for operator to connect to the mysql node for configuration
		out.Data["OPERATOR_USER"] = []byte("sys_operator")
		if err := addRandomPassword(out.Data, "OPERATOR_PASSWORD"); err != nil {
			return err
		}

		// the user that is used to configure replication between nodes
		out.Data["REPLICATION_USER"] = []byte("sys_replication")
		if err := addRandomPassword(out.Data, "REPLICATION_PASSWORD"); err != nil {
			return err
		}

		// the user that is used by the metrics exporter sidecar to collect mysql metrics
		out.Data["METRICS_EXPORTER_USER"] = []byte("sys_exporter")
		if err := addRandomPassword(out.Data, "METRICS_EXPORTER_PASSWORD"); err != nil {
			return err
		}

		// the user that is used by orchestrator to manage topology and failover
		out.Data["ORC_TOPOLOGY_USER"] = []byte(opt.OrchestratorTopologyUser)
		out.Data["ORC_TOPOLOGY_PASSWORD"] = []byte(opt.OrchestratorTopologyPassword)

		// the user that is used to serve backups over HTTP
		out.Data["BACKUP_USER"] = []byte("sys_backups")
		if err := addRandomPassword(out.Data, "BACKUP_PASSWORD"); err != nil {
			return err
		}

		return nil
	})
}

// addRandomPassword checks if a key exists and if not registers a random string for that key
func addRandomPassword(data map[string][]byte, key string) error {
	if len(data[key]) == 0 {
		// NOTE: use only alpha-numeric string, this strings are used unescaped in MySQL queries (issue #314)
		random, err := rand.AlphaNumericString(rStrLen)
		if err != nil {
			return err
		}
		data[key] = []byte(random)
	}
	return nil
}
