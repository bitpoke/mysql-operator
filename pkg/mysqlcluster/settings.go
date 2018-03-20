package mysqlcluster

import (
	"fmt"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
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
	}
	// MysqlMasterConfigs represents configs specific to master
	MysqlMasterConfigs = map[string]string{}
	// MysqlSlaveConfigs represents configs specific to slave
	MysqlSlaveConfigs = map[string]string{}
)

func (f *cFactory) getPodHostName(p int) string {
	pod := fmt.Sprintf("%s-%d", f.cl.GetNameForResource(api.StatefulSet), p)
	return fmt.Sprintf("%s.%s", pod, f.cl.GetNameForResource(api.HeadlessSVC))
}

func (f *cFactory) getLabels(extra map[string]string) map[string]string {
	lables := map[string]string{
		"app":              AppName,
		"titanium_cluster": f.cl.Name,
	}
	for k, v := range extra {
		lables[k] = v
	}
	return lables
}
