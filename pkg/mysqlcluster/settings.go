package mysqlcluster

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// AppName is the name of this application, it will be set as label for every resource
	AppName = "Titanium"
	// MysqlPortName represents the mysql port name.
	MysqlPortName = "mysql"
	// MysqlPort is the default mysql port.
	MysqlPort = 3306

	// TitaniumXtrabackupPortName is name of the port on which we take backups
	TitaniumXtrabackupPortName = "xtrabackup"
	// TitaniumXtrabackupPort is the port on which we serve backups
	TitaniumXtrabackupPort = 3307

	// OrcTopologyDir path where orc conf secret is mounted
	OrcTopologyDir = "/var/run/orc-topology"

	rStrLen = 18

	// ConfigVersion is the mysql config that needs to be updated if configs
	// change
	ConfigVersion = "2018-03-09:12:39"

	TitaniumProbePath = "/health"
	TitaniumProbePort = 8001
)

var (
	// TargetPort is the mysql port that is set for headless service and should be string
	TargetPort = intstr.FromInt(MysqlPort)

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

		// Utility user configs
		"utility-user-schema-access": "mysql",
		// TODO: least privileges principle
		"utility-user-privileges": "SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,GRANT,ALTER,SHOW DATABASES,SUPER,CREATE USER,PROCESS,RELOAD,LOCK TABLES,REPLICATION CLIENT,REPLICATION SLAVE",

		// MyISAM
		"key-buffer-size":        "32M",
		"myisam-recover-options": "FORCE,BACKUP",

		// Safety
		"max-allowed-packet": "16M",
		"max-connect-errors": "1000000",
		"skip-name-resolve":  "1",
		"sql-mode":           "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
		"sysdate-is-now":     "1",

		// binary logging
		"expire-logs-days": "14",
		"sync-binlog":      "1",

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
		"innodb-log-file-size":           "128M",
		"innodb-flush-log-at-trx-commit": "2",
		"innodb-file-per-table":          "1",
	}
)

func (f *cFactory) getLabels(extra map[string]string) map[string]string {
	lables := map[string]string{
		"app":              AppName,
		"titanium_cluster": f.cluster.Name,
	}
	for k, v := range extra {
		lables[k] = v
	}
	return lables
}
