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

package sidecar

import (
	"strconv"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

var (
	// MysqlServerIDOffset represents the offset with which all server ids are shifted from 0
	MysqlServerIDOffset = 100

	// MysqlPort represents port on which mysql works
	mysqlPort = strconv.Itoa(constants.MysqlPort)

	// ConfigDir is the mysql configs path, /etc/mysql
	configDir = constants.ConfVolumeMountPath

	// ConfDPath is /etc/mysql/conf.d
	confDPath = constants.ConfDPath

	// MountConfigDir is the mounted configs that needs processing
	mountConfigDir = constants.ConfMapVolumeMountPath

	// confClientPath the path where to put the client.cnf file
	confClientPath = constants.ConfClientPath

	// DataDir is the mysql data. /var/lib/mysql
	dataDir = constants.DataVolumeMountPath

	// ToolsDbName is the name of the tools table
	toolsDbName = constants.HelperDbName
	// ToolsInitTableName is the name of the init table
	toolsInitTableName = "init"

	// ServerPort http server port
	serverPort = constants.SidecarServerPort
	// ServerProbeEndpoint is the http server endpoint for probe
	serverProbeEndpoint = constants.SidecarServerProbePath
	// ServerBackupEndpoint is the http server endpoint for backups
	serverBackupEndpoint = "/xbackup"
)

const (
	// RcloneConfigFile represents the path to the file that contains rclon
	// configs. This path should be the same as defined in docker entrypoint
	// script from mysql-operator-sidecar/docker-entrypoint.sh. /etc/rclone.conf
	rcloneConfigFile = "/tmp/rclone.conf"
)
