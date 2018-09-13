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
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var log = logf.Log.WithName("config-map-syncer")

type configMapSyncer struct {
	cluster   *api.MysqlCluster
	configMap *core.ConfigMap
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

	"skip-name-resolve": "on",
	"skip-host-cache":   "on",
}

// NewConfigMapSyncer returns config map syncer
func NewConfigMapSyncer(cluster *api.MysqlCluster) syncer.Interface {

	obj := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(api.ConfigMap),
			Namespace: cluster.Namespace,
		},
	}

	return &configMapSyncer{
		cluster:   cluster,
		configMap: obj,
	}

}

func (s *configMapSyncer) GetObject() runtime.Object { return s.configMap }
func (s *configMapSyncer) GetOwner() runtime.Object  { return s.cluster }
func (s *configMapSyncer) GetEventReasonForError(err error) syncer.EventReason {
	return syncer.BasicEventReason("ConfigMap", err)
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
			log.V(2).Info(fmt.Sprintf("Skip updating configs, it's up to date: %s",
				out.ObjectMeta.Annotations["config_hash"]))
			return nil
		}
		log.V(2).Info(fmt.Sprintf("Config map hashes doesn't match: %s != %s. Updateing config map.", key, newHash))
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
		log.Error(err, "Can't compute hash for map data.")
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
				log.Error(err, fmt.Sprintf("Failed to add '%s':'%s' to config section", k, v))
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
