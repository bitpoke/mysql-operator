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
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// MysqlPortName represents the mysql port name.
	MysqlPortName = "mysql"
	// MysqlPort is the default mysql port.
	MysqlPort = 3306

	// HelperXtrabackupPortName is name of the port on which we take backups
	HelperXtrabackupPortName = "xtrabackup"
	// HelperXtrabackupPort is the port on which we serve backups
	HelperXtrabackupPort = 3307

	// OrcTopologyDir path where orc conf secret is mounted
	OrcTopologyDir = "/var/run/orc-topology"

	rStrLen = 18

	// ConfigVersion is the mysql config that needs to be updated if configs
	// change
	ConfigVersion = "2018-03-23:12:33"

	HelperProbePath = "/health"
	HelperProbePort = 8001

	ExporterPortName = "prometheus"
	ExporterPort     = 9104
	ExporterPath     = "/metrics"

	// HelperDbName represent the database name that is used by operator to
	// manage the mysql cluster. This database contains a table with
	// initialization history and table managed by pt-heartbeat. Be aware that
	// when changeing this value to update the orchestrator chart value for
	// SlaveLagQuery in hack/charts/mysql-operator/values.yaml.
	HelperDbName = "sys_operator"
)

var (
	// TargetPort is the mysql port that is set for headless service and should be string
	TargetPort = intstr.FromInt(MysqlPort)
	// ExporterTargetPort is the port on which metrics exporter expose metrics
	ExporterTargetPort = intstr.FromInt(ExporterPort)

	// MysqlMasterSlaveConfigs contains configs for both master and slave
	MysqlMasterSlaveConfigs = map[string]string{
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
		"sql-mode":           "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
		"sysdate-is-now":     "1",

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
	MysqlMasterSlaveBooleanConfigs = []string{
		// Safety
		"skip-name-resolve",
		"skip-host-cache",
	}
)

func (f *cFactory) getLabels(extra map[string]string) map[string]string {
	defaults_labels := f.cluster.GetLabels()
	for k, v := range extra {
		defaults_labels[k] = v
	}
	return defaults_labels
}
