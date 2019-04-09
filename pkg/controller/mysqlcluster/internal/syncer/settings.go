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
	"github.com/presslabs/mysql-operator/pkg/util/constants"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TODO: make those consts private and move them in the file where are used.
const (
	// MysqlPortName represents the mysql port name.
	MysqlPortName = "mysql"
	// MysqlPort is the default mysql port.
	MysqlPort = constants.MysqlPort

	// OrcTopologyDir path where orc conf secret is mounted
	OrcTopologyDir = constants.OrcTopologyDir

	// SidecarServerPortName name of the port
	SidecarServerPortName = "sidecar-http"
	// SidecarServerPort represents the port on which http server will run
	SidecarServerPort = constants.SidecarServerPort
	// SidecarServerProbePath the probe path
	SidecarServerProbePath = constants.SidecarServerProbePath

	// ExporterPort is the port that metrics will be exported
	ExporterPort = constants.ExporterPort
	//ExporterPortName the name of the metrics exporter port
	ExporterPortName = "prometheus"
	// ExporterPath is the path on which metrics are expose
	ExporterPath = constants.ExporterPath

	// HelperDbName represent the database name that is used by operator to
	// manage the mysql cluster.
	HelperDbName = constants.HelperDbName

	// ConfVolumeMountPath is the path where mysql configs will be mounted
	ConfVolumeMountPath = constants.ConfVolumeMountPath
	// DataVolumeMountPath is the path to mysql data
	DataVolumeMountPath = constants.DataVolumeMountPath
	// ConfMapVolumeMountPath represents the temp config mount path in init containers
	ConfMapVolumeMountPath = constants.ConfMapVolumeMountPath
	// ConfDPath is the path to extra mysql configs dir
	ConfDPath = constants.ConfDPath

	confClientPath = constants.ConfClientPath
)

var (
	// TargetPort is the mysql port that is set for headless service and should be string
	TargetPort = intstr.FromInt(MysqlPort)

	// ExporterTargetPort is the port for the exporter
	ExporterTargetPort = intstr.FromInt(ExporterPort)
)
