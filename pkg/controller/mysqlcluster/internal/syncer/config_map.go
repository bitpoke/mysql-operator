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
	"bytes"
	"fmt"
	"strconv"

	"github.com/go-ini/ini"
	"github.com/mitchellh/hashstructure"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var log = logf.Log.WithName("config-map-syncer")

type configMapSyncer struct {
	cluster *api.MysqlCluster
}

// NewConfigMapSyncer returns config map syncer
func NewConfigMapSyncer(c client.Client, scheme *runtime.Scheme, cluster *api.MysqlCluster) syncer.Interface {
	obj := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(api.ConfigMap),
			Namespace: cluster.Namespace,
		},
	}

	sync := &configMapSyncer{
		cluster: cluster,
	}

	return syncer.NewObjectSyncer("ConfigMap", cluster, obj, c, scheme, func(in runtime.Object) error {
		return sync.SyncFn(in)
	})
}

func (s *configMapSyncer) SyncFn(in runtime.Object) error {
	out := in.(*core.ConfigMap)

	out.ObjectMeta.Labels = s.cluster.GetLabels()
	out.ObjectMeta.Labels["generated"] = "true"

	data, newHash, err := s.getMysqlConfData()
	if err != nil {
		return fmt.Errorf("failed to create mysql configs, err: %s", err)
	}

	if key, ok := out.ObjectMeta.Annotations["config_hash"]; ok {
		if key == newHash {
			log.V(2).Info("skip updating configs", "hash", out.ObjectMeta.Annotations["config_hash"])
			return nil
		}
		log.V(2).Info("config map hashes don't match", "old_hash", key, "new_hash", newHash)
	}

	out.ObjectMeta.Annotations = map[string]string{
		"config_hash": newHash,
	}

	out.Data = map[string]string{
		"my.cnf": data,
	}

	return nil
}

func (s *configMapSyncer) getMysqlConfData() (string, string, error) {
	cfg := ini.Empty()
	sec := cfg.Section("mysqld")

	addKVConfigsToSection(sec, mysqlMasterSlaveConfigs, s.cluster.Spec.MysqlConf)
	addBConfigsToSection(sec, mysqlMasterSlaveBooleanConfigs)

	// include configs from /etc/mysql/conf.d/*.cnf
	_, err := sec.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))
	if err != nil {
		return "", "", err
	}

	data, err := writeConfigs(cfg)
	if err != nil {
		return "", "", err
	}
	hash, err := hashstructure.Hash(cfg.Section("mysqld").KeysHash(), nil)
	if err != nil {
		log.Error(err, "can't compute hash for map data.")
		return "", "", err
	}

	newHash := strconv.FormatUint(hash, 10)
	return data, newHash, nil

}

// helper function to add a map[string]string to a ini.Section
func addKVConfigsToSection(s *ini.Section, extraMysqld ...map[string]string) {
	for _, extra := range extraMysqld {
		for k, v := range extra {
			if _, err := s.NewKey(k, v); err != nil {
				log.Error(err, "failed to add key to config section", "key", k, "value", v, "section", s)
			}
		}
	}
}

// helper function to add a string to a ini.Section
func addBConfigsToSection(s *ini.Section, boolConfigs ...[]string) {
	for _, extra := range boolConfigs {
		for _, k := range extra {
			if _, err := s.NewBooleanKey(k); err != nil {
				log.Error(err, "failed to add boolean key to config section", "key", k)
			}
		}
	}
}

// helper function to write to string ini.File
// nolint: interfacer
func writeConfigs(cfg *ini.File) (string, error) {
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// mysqlMasterSlaveConfigs represents the configuration that mysql-operator needs by default
var mysqlMasterSlaveConfigs = map[string]string{
	"log-bin":           "/var/lib/mysql/mysql-bin",
	"log-slave-updates": "on",

	"read-only":        "on",
	"skip-slave-start": "on",

	// Crash safe
	"relay-log-info-repository": "TABLE",
	"relay-log-recovery":        "on",

	// https://github.com/github/orchestrator/issues/323#issuecomment-338451838
	"master_info_repository": "TABLE",

	"default-storage-engine":   "InnoDB",
	"gtid-mode":                "on",
	"enforce-gtid-consistency": "on",

	// MyISAM
	"key-buffer-size":        "32M",
	"myisam-recover-options": "FORCE,BACKUP",

	// Safety
	"max-allowed-packet": "16M",
	"max-connect-errors": "1000000",
	"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER," +
		"NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
	"sysdate-is-now": "1",

	// Binary logging
	"expire-logs-days": "14",
	"sync-binlog":      "1",
	"binlog-format":    "ROW",

	// CACHES AND LIMITS
	"tmp-table-size":         "32M",
	"max-heap-table-size":    "32M",
	"query-cache-type":       "0",
	"query-cache-size":       "0",
	"max-connections":        "500",
	"thread-cache-size":      "50",
	"open-files-limit":       "65535",
	"table-definition-cache": "4096",
	"table-open-cache":       "4096",

	// InnoDB
	"innodb-flush-method":            "O_DIRECT",
	"innodb-log-files-in-group":      "2",
	"innodb-flush-log-at-trx-commit": "2",
	"innodb-file-per-table":          "1",

	"character-set-server": "utf8mb4",
	"collation-server":     "utf8mb4_unicode_ci",
}

var mysqlMasterSlaveBooleanConfigs = []string{
	// Safety
	"skip-name-resolve",
	"skip-host-cache",
}
