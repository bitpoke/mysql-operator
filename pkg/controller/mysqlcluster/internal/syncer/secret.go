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

// NewSecretSyncer returns secret syncer
// nolint: gocyclo
func NewSecretSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, opt *options.Options) syncer.Interface {
	obj := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Spec.SecretName,
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("Secret", nil, obj, c, scheme, func(in runtime.Object) error {
		out := in.(*core.Secret)

		if _, ok := out.Data["ROOT_PASSWORD"]; !ok {
			return fmt.Errorf("ROOT_PASSWORD not set in secret: %s", out.Name)
		}

		if len(out.Data["REPLICATION_USER"]) == 0 {
			random, err := rand.AlphaNumericString(5)
			if err != nil {
				return err
			}
			out.Data["REPLICATION_USER"] = []byte("repl_" + random)
		}
		if len(out.Data["REPLICATION_PASSWORD"]) == 0 {
			random, err := rand.ASCIIString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["REPLICATION_PASSWORD"] = []byte(random)
		}
		if len(out.Data["METRICS_EXPORTER_USER"]) == 0 {
			random, err := rand.AlphaNumericString(5)
			if err != nil {
				return err
			}
			out.Data["METRICS_EXPORTER_USER"] = []byte("exp_" + random)
		}
		if len(out.Data["METRICS_EXPORTER_PASSWORD"]) == 0 {
			random, err := rand.ASCIIString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["METRICS_EXPORTER_PASSWORD"] = []byte(random)
		}

		out.Data["ORC_TOPOLOGY_USER"] = []byte(opt.OrchestratorTopologyUser)
		out.Data["ORC_TOPOLOGY_PASSWORD"] = []byte(opt.OrchestratorTopologyPassword)

		if len(out.Data["BACKUP_USER"]) == 0 {
			random, err := rand.AlphaNumericString(rStrLen)
			if err != nil {
				return err
			}
			out.Data["BACKUP_USER"] = []byte(random)
		}

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
