/*
Copyright 2019 Pressinfra SRL

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
	"fmt"
	"os"
	"strconv"
	"strings"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/controller-util/rand"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

// NodeRole represents the kind of the MySQL server
type NodeRole string

const (
	// MasterNode represents the master role for MySQL server
	MasterNode NodeRole = "master"
	// SlaveNode represents the slave role for MySQL server
	SlaveNode NodeRole = "slave"
)

// Config contains information related with the pod.
type Config struct {
	// Hostname represents the pod hostname
	Hostname string
	// ClusterName is the MySQL cluster name
	ClusterName string
	// Namespace represents the namespace where the pod is in
	Namespace string
	// ServiceName is the name of the headless service
	ServiceName string

	// InitBucketURL represents the init bucket to initialize mysql
	InitBucketURL string

	// OrchestratorURL is the URL to connect to orchestrator
	OrchestratorURL string

	// OperatorUser represents the credentials that the operator will use to connect to the mysql
	OperatorUser     string
	OperatorPassword string

	// backup user and password for http endpoint
	BackupUser     string
	BackupPassword string

	// replication user and password
	ReplicationUser     string
	ReplicationPassword string

	// metrics exporter user and password
	MetricsUser     string
	MetricsPassword string

	// orchestrator credentials
	OrchestratorUser     string
	OrchestratorPassword string

	// heartbeat credentials
	HeartBeatUser     string
	HeartBeatPassword string
}

// FQDNForServer returns the pod hostname for given MySQL server id
func (cfg *Config) FQDNForServer(id int) string {
	base := mysqlcluster.GetNameForResource(mysqlcluster.StatefulSet, cfg.ClusterName)
	return fmt.Sprintf("%s-%d.%s.%s", base, id-MysqlServerIDOffset, cfg.ServiceName, cfg.Namespace)
}

func (cfg *Config) newOrcClient() orc.Interface {
	if len(cfg.OrchestratorURL) == 0 {
		log.Info("OrchestratorURL not set")
		return nil
	}

	return orc.NewFromURI(cfg.OrchestratorURL)
}

// ClusterFQDN returns the cluster FQ Name of the cluster from which the node belongs
func (cfg *Config) ClusterFQDN() string {
	return fmt.Sprintf("%s.%s", cfg.ClusterName, cfg.Namespace)
}

// MasterFQDN the FQ Name of the cluster's master
func (cfg *Config) MasterFQDN() string {
	if client := cfg.newOrcClient(); client != nil {
		if master, err := client.Master(cfg.ClusterFQDN()); err == nil {
			return master.Key.Hostname
		}
	}

	log.V(-1).Info("failed to obtain master from orchestrator, go for default master",
		"master", cfg.FQDNForServer(MysqlServerIDOffset))
	return cfg.FQDNForServer(MysqlServerIDOffset)

}

// NodeRole returns the role of the current node
func (cfg *Config) NodeRole() NodeRole {
	if cfg.FQDNForServer(cfg.ServerID()) == cfg.MasterFQDN() {
		return MasterNode
	}

	return SlaveNode
}

// ServerID returns the MySQL server id
func (cfg *Config) ServerID() int {
	ordinal := getOrdinalFromHostname(cfg.Hostname)
	return ordinal + MysqlServerIDOffset
}

// MysqlDSN returns the connection string to MySQL server
func (cfg *Config) MysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%s)/?timeout=5s&multiStatements=true&interpolateParams=true",
		cfg.OperatorUser, cfg.OperatorPassword, mysqlPort,
	)
}

// NewConfig returns a pointer to Config configured from environment variables
func NewConfig() *Config {
	hbPass, err := rand.AlphaNumericString(10)
	if err != nil {
		panic(err)
	}

	cfg := &Config{
		Hostname:    getEnvValue("HOSTNAME"),
		ClusterName: getEnvValue("MY_CLUSTER_NAME"),
		Namespace:   getEnvValue("MY_NAMESPACE"),
		ServiceName: getEnvValue("MY_SERVICE_NAME"),

		InitBucketURL:   getEnvValue("INIT_BUCKET_URI"),
		OrchestratorURL: getEnvValue("ORCHESTRATOR_URI"),

		OperatorUser:     getEnvValue("OPERATOR_USER"),
		OperatorPassword: getEnvValue("OPERATOR_PASSWORD"),

		BackupUser:     getEnvValue("BACKUP_USER"),
		BackupPassword: getEnvValue("BACKUP_PASSWORD"),

		ReplicationUser:     getEnvValue("REPLICATION_USER"),
		ReplicationPassword: getEnvValue("REPLICATION_PASSWORD"),

		MetricsUser:     getEnvValue("METRICS_EXPORTER_USER"),
		MetricsPassword: getEnvValue("METRICS_EXPORTER_PASSWORD"),

		OrchestratorUser:     getEnvValue("ORC_TOPOLOGY_USER"),
		OrchestratorPassword: getEnvValue("ORC_TOPOLOGY_PASSWORD"),

		HeartBeatUser:     heartBeatUserName,
		HeartBeatPassword: hbPass,
	}

	return cfg
}

func getEnvValue(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		log.Info("environment is not set", "key", key)
	}

	return value
}

func getOrdinalFromHostname(hn string) int {
	// mysql-master-1
	// or
	// stateful-ceva-3
	l := strings.Split(hn, "-")
	for i := len(l) - 1; i >= 0; i-- {
		if o, err := strconv.ParseInt(l[i], 10, 8); err == nil {
			return int(o)
		}
	}

	return 0
}
