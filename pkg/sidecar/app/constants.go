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

package app

import (
	"strconv"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

var (
	// MysqlPort represents port on which mysql works
	MysqlPort = strconv.Itoa(constants.MysqlPort)

	// ConfigDir is the mysql configs path, /etc/mysql
	ConfigDir = constants.ConfVolumeMountPath

	// ConfDPath is /etc/mysql/conf.d
	ConfDPath = constants.ConfDPath

	// MountConfigDir is the mounted configs that needs processing
	MountConfigDir = constants.ConfMapVolumeMountPath

	// DataDir is the mysql data. /var/lib/mysql
	DataDir = constants.DataVolumeMountPath

	// ToolsDbName is the name of the tools table
	ToolsDbName = constants.HelperDbName
	// ToolsInitTableName is the name of the init table
	ToolsInitTableName = "init"

	// UtilityUser is the name of the percona utility user.
	UtilityUser = "sys_utility_sidecar"

	// OrcTopologyDir contains the path where the secret with orc credentials is
	// mounted.
	OrcTopologyDir = constants.OrcTopologyDir

	// ServerPort http server port
	ServerPort = constants.SidecarServerPort
	// ServerProbeEndpoint is the http server endpoint for probe
	ServerProbeEndpoint = constants.SidecarServerProbePath
	// ServerBackupEndpoint is the http server endpoint for backups
	ServerBackupEndpoint = "/xbackup"
)

const (
	// RcloneConfigFile represents the path to the file that contains rclon
	// configs. This path should be the same as defined in docker entrypoint
	// script from mysql-operator-sidecar/docker-entrypoint.sh. /etc/rclone.conf
	RcloneConfigFile = "/etc/rclone.conf"
)
