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

	// ConfigVersion is the mysql config that needs to be updated if configs
	// change
	ConfigVersion = "2018-03-23:12:33"

	// HelperServerPort represents the port on which http server will run
	HelperServerPort = 8088
	// HelperServerProbePath the probe path
	HelperServerProbePath = "/health"

	//ExporterPortName the name of the metrics exporter port
	ExporterPortName = "prometheus"

	// ExporterPath is the path on which metrics are expose
	ExporterPath = "/metrics"

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
)
