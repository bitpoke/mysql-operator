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
	"sort"
	"strings"

	"github.com/go-ini/ini"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/presslabs/controller-util/syncer"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

var log = logf.Log.WithName("config-map-syncer")

// NewConfigMapSyncer returns config map syncer
func NewConfigMapSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster) syncer.Interface {
	cm := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.ConfigMap),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("ConfigMap", cluster.Unwrap(), cm, c, scheme, func() error {
		cm.ObjectMeta.Labels = cluster.GetLabels()
		cm.ObjectMeta.Labels["generated"] = "true"

		data, err := buildMysqlConfData(cluster)
		if err != nil {
			return fmt.Errorf("failed to create mysql configs: %s", err)
		}

		preStopSh := buildBashPreStop()

		cm.Data = map[string]string{
			"my.cnf":      data,
			shPreStopFile: preStopSh,
		}

		return nil
	})
}

func buildBashPreStop() string {
	data := `#!/bin/bash
set -ex

current=$(date "+%Y-%m-%d %H:%M:%S")
echo "[${current}]preStop is ongoing"
read_only_status=$(mysql --defaults-file=ConfClientPathHolder -NB -e 'SELECT @@read_only')
replica_status=$(mysql --defaults-file=ConfClientPathHolder -NB -e 'show slave status\G')
# orchestrator will isolate old master during failover
has_replica_hosts=$(mysql --defaults-file=ConfClientPathHolder -NB -e 'show slave hosts\G')
replica_status_count=$(echo -n "$replica_status" | wc -l )
has_replica_count=$(echo -n "$has_replica_hosts" | wc -l )
echo "hostname=$(hostname) readonly=${read_only_status} show_slave_status=${replica_status_count}"
echo "has_replica_hosts=${has_replica_count}"
if [ ${read_only_status} -eq 0  ] && [ ${replica_status_count} -eq 0 ] && [ ${has_replica_count} -gt 0 ]
then
		masterhostname=$( curl  -s "${ORCH_HTTP_API}/master/${ORCH_CLUSTER_ALIAS}" |  awk -F":" '{print $3}' | awk -F'"' '{print $2}' )
        echo "master from orchestrator: ${masterhostname}"
        if [ "${FQDN}" == "${masterhostname}" ]
        then
                curl  -s "${ORCH_HTTP_API}/graceful-master-takeover-auto/${ORCH_CLUSTER_ALIAS}"
				echo "graceful-master-takeover-auto is ongoing, sleep 5 seconds in order to make sure service can work well."
				sleep 5
        fi
fi
`
	data = strings.Replace(data, "ConfClientPathHolder", confClientPath, -1)
	return data
}

func buildMysqlConfData(cluster *mysqlcluster.MysqlCluster) (string, error) {
	cfg := ini.Empty()
	sec := cfg.Section("mysqld")

	addKVConfigsToSection(sec, getMysqlMasterSlaveConfigs(), cluster.Spec.MysqlConf)
	addBConfigsToSection(sec, mysqlMasterSlaveBooleanConfigs)

	// include configs from /etc/mysql/conf.d/*.cnf
	_, err := sec.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))
	if err != nil {
		return "", err
	}

	data, err := writeConfigs(cfg)
	if err != nil {
		return "", err
	}

	return data, nil

}

func getMysqlMasterSlaveConfigs() map[string]intstr.IntOrString {
	config := make(map[string]intstr.IntOrString)

	for key, value := range mysqlMasterSlaveConfigs {
		config[key] = intstr.Parse(value)
	}

	return config
}

// helper function to add a map[string]string to a ini.Section
func addKVConfigsToSection(s *ini.Section, extraMysqld ...map[string]intstr.IntOrString) {
	for _, extra := range extraMysqld {
		keys := []string{}
		for key := range extra {
			keys = append(keys, key)
		}

		// sort keys
		sort.Strings(keys)

		for _, k := range keys {
			value := extra[k]
			if _, err := s.NewKey(k, value.String()); err != nil {
				log.Error(err, "failed to add key to config section", "key", k, "value", extra[k], "section", s)
			}
		}
	}
}

// helper function to add a string to a ini.Section
func addBConfigsToSection(s *ini.Section, boolConfigs ...[]string) {
	for _, extra := range boolConfigs {
		keys := []string{}
		keys = append(keys, extra...)

		// sort keys
		sort.Strings(keys)

		for _, k := range keys {
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
	"master-info-repository": "TABLE",

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
		"NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
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
