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

package constants

const (
	// MysqlPort is the default mysql port.
	MysqlPort = 3306

	// OrcTopologyDir path where orc conf secret is mounted
	OrcTopologyDir = "/var/run/orc-topology"

	// SidecarServerPort represents the port on which http server will run
	SidecarServerPort = 8080
	// SidecarServerProbePath the probe path
	SidecarServerProbePath = "/health"

	// ExporterPort is the port that metrics will be exported
	ExporterPort = 9125

	// ExporterPath is the path on which metrics are expose
	ExporterPath = "/metrics"

	// OperatorDbName represent the database name that is used by operator to
	// manage the mysql cluster. This database contains a table with
	// initialization history and table managed by pt-heartbeat. Be aware that
	// when changing this value to update the orchestrator chart value for
	// SlaveLagQuery in hack/charts/mysql-operator/values.yaml.
	OperatorDbName = "sys_operator"

	// OperatorInitTableName represents the name of the table that is used to mark configuration complete
	OperatorInitTableName = "init"

	// ConfVolumeMountPath is the path where mysql configs will be mounted
	ConfVolumeMountPath = "/etc/mysql"

	// InitDBVolumeMountPath the path where to put *.sql init files. Docker entrypoint runs the scripts from this path.
	InitDBVolumeMountPath = "/docker-entrypoint-initdb.d/"

	// DataVolumeMountPath is the path to mysql data
	DataVolumeMountPath = "/var/lib/mysql"

	// ConfMapVolumeMountPath represents the temp config mount path in init containers
	ConfMapVolumeMountPath = "/mnt/conf"

	// ConfDPath is the path to extra mysql configs dir
	ConfDPath = "/etc/mysql/conf.d"

	// ConfClientPath represents the path to the client MySQL client configuration
	// it's important to have a different extension than .cnf to be ignore by MySQL include
	ConfClientPath = "/etc/mysql/client.conf"

	// ConfHeartBeatPath the path where to put the heartbeat.conf file
	// it's important to have a different extension than .cnf to be ignore by MySQL include
	ConfHeartBeatPath = "/etc/mysql/heartbeat.conf"
)

var (
	// MysqlImageVersions is a map of supported mysql version and their image
	MysqlImageVersions = map[string]string{
		// percona:5.7.24 centos based image
		"5.7": "percona@sha256:b3b7fb177b416563c46fe012298e042ec1607cc0539ce6014146380b0d27b08c",
	}
)
