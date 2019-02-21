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

package app

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-ini/ini"
	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

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

// BaseConfig contains information related with the pod.
type BaseConfig struct {
	StopCh <-chan struct{}

	// Hostname represents the pod hostname
	Hostname string
	// ClusterName is the MySQL cluster name
	ClusterName string
	// Namespace represents the namespace where the pod is in
	Namespace string
	// ServiceName is the name of the headless service
	ServiceName string

	// NodeRole represents the MySQL role of the node, can be on of: msater, slave
	NodeRole NodeRole
	// ServerID represents the MySQL server id
	ServerID int

	// InitBucketURL represents the init bucket to initialize mysql
	InitBucketURL *string

	// OrchestratorURL is the URL to connect to orchestrator
	OrchestratorURL *string

	// MasterHost represents the cluster master hostname
	MasterHost string

	// backup user and password for http endpoint
	BackupUser     string
	BackupPassword string
}

// GetHostFor returns the pod hostname for given MySQL server id
func (cfg *BaseConfig) GetHostFor(id int) string {
	base := mysqlcluster.GetNameForResource(mysqlcluster.StatefulSet, cfg.ClusterName)
	return fmt.Sprintf("%s-%d.%s.%s", base, id-100, cfg.ServiceName, cfg.Namespace)
}

func (cfg *BaseConfig) getOrcClient() orc.Interface {
	if cfg.OrchestratorURL == nil {
		return nil
	}

	return orc.NewFromURI(*cfg.OrchestratorURL)
}

func (cfg *BaseConfig) getFQClusterName() string {
	return fmt.Sprintf("%s.%s", cfg.ClusterName, cfg.Namespace)
}

func (cfg *BaseConfig) getMasterHost() string {
	if client := cfg.getOrcClient(); client != nil {
		if master, err := client.Master(cfg.getFQClusterName()); err == nil {
			return master.Key.Hostname
		}
	}

	log.V(-1).Info("failed to obtain master from orchestrator, go for default master",
		"master", cfg.GetHostFor(100))
	return cfg.GetHostFor(100)

}

// NewBasicConfig returns a pointer to BaseConfig configured from environment variables
func NewBasicConfig(stop <-chan struct{}) *BaseConfig {
	cfg := &BaseConfig{
		StopCh:      stop,
		Hostname:    getEnvValue("HOSTNAME"),
		ClusterName: getEnvValue("MY_CLUSTER_NAME"),
		Namespace:   getEnvValue("MY_NAMESPACE"),
		ServiceName: getEnvValue("MY_SERVICE_NAME"),

		InitBucketURL:   getEnvP("INIT_BUCKET_URI"),
		OrchestratorURL: getEnvP("ORCHESTRATOR_URI"),

		BackupUser:     getEnvValue("MYSQL_BACKUP_USER"),
		BackupPassword: getEnvValue("MYSQL_BACKUP_PASSWORD"),
	}

	// get master host
	cfg.MasterHost = cfg.getMasterHost()

	// set node role
	cfg.NodeRole = SlaveNode
	if cfg.Hostname == cfg.MasterHost {
		cfg.NodeRole = MasterNode
	}

	// get server id
	ordinal := getOrdinalFromHostname(cfg.Hostname)
	cfg.ServerID = ordinal + 100

	return cfg
}

func getEnvValue(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		log.Info("envirorment is not set", "key", key)
	}

	return value
}

func getEnvP(key string) *string {
	if value := getEnvValue(key); len(value) != 0 {
		return &value
	}
	return nil
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

// MysqlConfig contains extra information to connect or configure MySQL server
type MysqlConfig struct {
	// inherit from base config
	BaseConfig

	MysqlDSN *string

	// replication user and password
	ReplicationUser     string
	ReplicationPassword string

	// metrcis exporter user and password
	MetricsUser     string
	MetricsPassword string

	// orchestrator credentials
	OrchestratorUser     string
	OrchestratorPassword string
}

// NewMysqlConfig returns a pointer to MysqlConfig
func NewMysqlConfig(cfg *BaseConfig) *MysqlConfig {
	mycfg := &MysqlConfig{
		BaseConfig: *cfg,

		ReplicationUser:     getEnvValue("MYSQL_REPLICATION_USER"),
		ReplicationPassword: getEnvValue("MYSQL_REPLICATION_PASSWORD"),

		MetricsUser:     getEnvValue("MYSQL_METRICS_EXPORTER_USER"),
		MetricsPassword: getEnvValue("MYSQL_METRICS_EXPORTER_PASSWORD"),

		OrchestratorUser:     getEnvValue("MYSQL_ORC_TOPOLOGY_USER"),
		OrchestratorPassword: getEnvValue("MYSQL_ORC_TOPOLOGY_PASSWORD"),
	}

	// set connection DSN to MySQL
	var err error
	if mycfg.MysqlDSN, err = getMySQLConnectionString(); err != nil {
		log.Error(err, "get MySQL DSN")
	}

	return mycfg
}

// getMySQLConnectionString returns the mysql DSN
func getMySQLConnectionString() (*string, error) {
	cnfPath := path.Join(ConfigDir, "client.cnf")
	cfg, err := ini.Load(cnfPath)
	if err != nil {
		return nil, fmt.Errorf("Could not open %s: %s", cnfPath, err)
	}

	client := cfg.Section("client")
	host := client.Key("host").String()
	user := client.Key("user").String()
	password := client.Key("password").String()
	port, err := client.Key("port").Int()

	if err != nil {
		return nil, fmt.Errorf("Invalid port in %s: %s", cnfPath, err)
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		user, password, host, port,
	)
	return &dsn, nil
}
