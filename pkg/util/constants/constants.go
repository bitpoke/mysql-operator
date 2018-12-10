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

	// HelperXtrabackupPort is the port on which we serve backups
	HelperXtrabackupPort = 3307

	// OrcTopologyDir path where orc conf secret is mounted
	OrcTopologyDir = "/var/run/orc-topology"

	// HelperServerPort represents the port on which http server will run
	HelperServerPort = 8088
	// HelperServerProbePath the probe path
	HelperServerProbePath = "/health"

	// ExporterPort is the port that metrics will be exported
	ExporterPort = 9125

	// ExporterPath is the path on which metrics are expose
	ExporterPath = "/metrics"

	// HelperDbName represent the database name that is used by operator to
	// manage the mysql cluster. This database contains a table with
	// initialization history and table managed by pt-heartbeat. Be aware that
	// when changeing this value to update the orchestrator chart value for
	// SlaveLagQuery in hack/charts/mysql-operator/values.yaml.
	HelperDbName = "sys_operator"

	// ConfVolumeMountPath is the path where mysql configs will be mounted
	ConfVolumeMountPath = "/etc/mysql"

	// DataVolumeMountPath is the path to mysql data
	DataVolumeMountPath = "/var/lib/mysql"

	// ConfMapVolumeMountPath represents the temp config mount path in init containers
	ConfMapVolumeMountPath = "/mnt/conf"

	// ConfDPath is the path to extra mysql configs dir
	ConfDPath = "/etc/mysql/conf.d"
)

var (
	// MysqlImageVersions is a map of supported mysql version and their image
	MysqlImageVersions = map[string]string{
		// TODO: modify operator to use percona centos images
		// percona:5.7-stretch
		"5.7": "percona@sha256:c8b69b3c753cb04f1cbf8a4a1f295f51675761ee6368a47808a5205e2d45cfeb",
	}
)
