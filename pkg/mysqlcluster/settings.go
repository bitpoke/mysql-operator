package mysqlcluster

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// Headless Service
	HeadlessServiceName = "headless"
	AppName             = "Titanium"
	MysqlPortName       = "mysql"
	MysqlPort           = 3306

	// config
	TitaniumXtrabackupPortName = "xtrabackup"
	TitaniumXtrabackupPort     = 3307
)

var (
	// Headless Service
	TargetPort = intstr.FromInt(MysqlPort)

	// mysql configs
	MysqlMasterSlaveConfigs = map[string]string{
		"default-storage-engine":   "InnoDB",
		"gtid-mode":                "on",
		"enforce-gtid-consistency": "on",
	}
	MysqlMasterConfigs = map[string]string{
		"log-bin": "/var/lib/mysql/mysql-bin",
	}
	MysqlSlaveConfigs = map[string]string{
		"super-read-only": "on",
		// Crash safe
		"relay-log-info-repository": "TABLE",
		"relay-log-recovery":        "on",
	}

	EnvConfigSecret = map[string]string{}
)

type ResourceName string

const (
	HeadlessSVC ResourceName = "headless"
	StatefulSet ResourceName = "mysql"
	ConfigMap   ResourceName = "config-files"
	EnvSecret   ResourceName = "env-config"
	VolumePVC   ResourceName = "mysql-data"
	SSPod       ResourceName = "pod"
	DbSecret    ResourceName = "db-credentials"
)

func (c *cluster) getNameForResource(name ResourceName) string {
	return fmt.Sprintf("%s-%s", c.cl.Name, name)
}

func (c *cluster) getPorHostName(p int) string {
	pod := fmt.Sprintf("%s-%d", c.getNameForResource(StatefulSet), p)
	return fmt.Sprintf("%s.%s", pod, c.getNameForResource(HeadlessSVC))
}

func (c *cluster) getLabels(extra map[string]string) map[string]string {
	lables := map[string]string{
		"app":              AppName,
		"titanium_cluster": c.cl.Name,
	}
	for k, v := range extra {
		lables[k] = v
	}
	return lables
}

// can be a utitilit function
// can be removed!
func (c *cluster) getResourceRequirements(requests, limits map[string]string) apiv1.ResourceRequirements {
	reqResourceList := make(apiv1.ResourceList)
	for k, v := range requests {
		res := apiv1.ResourceName(k)
		reqResourceList[res] = resource.MustParse(v)
	}
	limResourceList := make(apiv1.ResourceList)
	for k, v := range limits {
		res := apiv1.ResourceName(k)
		limResourceList[res] = resource.MustParse(v)
	}

	return apiv1.ResourceRequirements{
		Requests: reqResourceList,
		Limits:   limResourceList,
	}
}
