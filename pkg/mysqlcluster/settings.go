package mysqlcluster

import (
	"fmt"

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
)

var (
	// TargetPort is the mysql port that is set for headless service and should be string
	TargetPort = intstr.FromInt(MysqlPort)

	// MysqlMasterSlaveConfigs contains configs for both master and slave
	MysqlMasterSlaveConfigs = map[string]string{
		"default-storage-engine":   "InnoDB",
		"gtid-mode":                "on",
		"enforce-gtid-consistency": "on",
	}
	// MysqlMasterConfigs represents configs specific to master
	MysqlMasterConfigs = map[string]string{
		"log-bin": "/var/lib/mysql/mysql-bin",
	}
	// MysqlSlaveConfigs represents configs specific to slave
	MysqlSlaveConfigs = map[string]string{
		"super-read-only": "on",
		// Crash safe
		"relay-log-info-repository": "TABLE",
		"relay-log-recovery":        "on",
	}
)

// ResourceName is the type for aliasing resources that will be created.
type ResourceName string

const (
	// HeadlessSVC is the alias of the headless service resource
	HeadlessSVC ResourceName = "headless"
	// StatefulSet is the alias of the statefulset resource
	StatefulSet ResourceName = "mysql"
	// ConfigMap is the alias for mysql configs, the config map resource
	ConfigMap ResourceName = "config-files"
	// VolumePVC is the alias of the PVC volume
	VolumePVC ResourceName = "mysql-data"
	// EnvSecret is the alias for secret that contains env variables
	EnvSecret ResourceName = "env-config"
	// UtilitySecret is the alias for utility secret
	UtilitySecret ResourceName = "utility"
)

func (f *cFactory) getNameForResource(name ResourceName) string {
	return fmt.Sprintf("%s-%s", f.cl.Name, name)
}

func (f *cFactory) getPorHostName(p int) string {
	pod := fmt.Sprintf("%s-%d", f.getNameForResource(StatefulSet), p)
	return fmt.Sprintf("%s.%s", pod, f.getNameForResource(HeadlessSVC))
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
