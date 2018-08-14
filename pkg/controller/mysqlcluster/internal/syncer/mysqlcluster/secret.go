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

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/mysqlcluster/internal/syncer"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util"
)

const (
	rStrLen = 18
)

type secretSyncer struct {
	cluster *api.MysqlCluster
	opt     *options.Options
}

// NewSecretSyncer returns secret syncer
func NewSecretSyncer(cluster *api.MysqlCluster) syncer.Interface {
	return &secretSyncer{
		cluster: cluster,
		opt:     options.GetOptions(),
	}
}

func (s *secretSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.cluster.Spec.SecretName,
			Namespace: s.cluster.Namespace,
		},
	}
}

func (s *secretSyncer) ShouldHaveOwnerReference() bool {
	return false
}

func (s *secretSyncer) Sync(in runtime.Object) error {
	out := in.(*core.Secret)

	if _, ok := out.Data["ROOT_PASSWORD"]; !ok {
		return fmt.Errorf("ROOT_PASSWORD not set in secret: %s", out.Name)
	}

	if len(out.Data["REPLICATION_USER"]) == 0 {
		out.Data["REPLICATION_USER"] = []byte("repl_" + util.RandStringUser(5))
	}
	if len(out.Data["REPLICATION_PASSWORD"]) == 0 {
		out.Data["REPLICATION_PASSWORD"] = []byte(util.RandomString(rStrLen))
	}
	if len(out.Data["METRICS_EXPORTER_USER"]) == 0 {
		out.Data["METRICS_EXPORTER_USER"] = []byte("repl_" + util.RandStringUser(5))
	}
	if len(out.Data["METRICS_EXPORTER_PASSWORD"]) == 0 {
		out.Data["METRICS_EXPORTER_PASSWORD"] = []byte(util.RandomString(rStrLen))
	}

	out.Data["ORC_TOPOLOGY_USER"] = []byte(s.opt.OrchestratorTopologyUser)
	out.Data["ORC_TOPOLOGY_PASSWORD"] = []byte(s.opt.OrchestratorTopologyPassword)

	if len(out.Data["BACKUP_USER"]) == 0 {
		out.Data["BACKUP_USER"] = []byte(util.RandomString(rStrLen))
	}

	if len(out.Data["BACKUP_PASSWORD"]) == 0 {
		out.Data["BACKUP_PASSWORD"] = []byte(util.RandomString(rStrLen))
	}

	return nil
}
