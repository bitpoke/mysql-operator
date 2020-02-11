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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/controller-util/rand"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
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

	// ExistsMySQLData checks if MySQL data is initialized by checking if the mysql dir exists
	ExistsMySQLData bool

	// Offset for assigning MySQL Server ID
	MyServerIDOffset int

	// XbstreamExtraArgs is a list of extra command line arguments to pass to xbstream.
	XbstreamExtraArgs []string

	masterService              string
	healthyReplicaCloneService string
}

// FQDNForServer returns the pod hostname for given MySQL server id
func (cfg *Config) FQDNForServer(id int) string {
	base := mysqlcluster.GetNameForResource(mysqlcluster.StatefulSet, cfg.ClusterName)
	return fmt.Sprintf("%s-%d.%s.%s", base, id-cfg.MyServerIDOffset, cfg.ServiceName, cfg.Namespace)
}

// ClusterFQDN returns the cluster FQ Name of the cluster from which the node belongs
func (cfg *Config) ClusterFQDN() string {
	return fmt.Sprintf("%s.%s", cfg.ClusterName, cfg.Namespace)
}

// MasterFQDN the FQ Name of the cluster's master
func (cfg *Config) MasterFQDN() string {
	if cfg.masterService != "" {
		return cfg.masterService
	}
	return mysqlcluster.GetNameForResource(mysqlcluster.MasterService, cfg.ClusterName)
}

// ReplicasFQDN the FQ Name of the replicas service
func (cfg *Config) ReplicasFQDN() string {
	if cfg.healthyReplicaCloneService != "" {
		return cfg.healthyReplicaCloneService
	}
	return mysqlcluster.GetNameForResource(mysqlcluster.HealthyReplicasService, cfg.ClusterName)
}

// ServerID returns the MySQL server id
func (cfg *Config) ServerID() int {
	ordinal := getOrdinalFromHostname(cfg.Hostname)
	return ordinal + cfg.MyServerIDOffset
}

// MysqlDSN returns the connection string to MySQL server
func (cfg *Config) MysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%s)/?timeout=5s&multiStatements=true&interpolateParams=true",
		cfg.OperatorUser, cfg.OperatorPassword, mysqlPort,
	)
}

// IsFirstPodInSet returns true if this pod has an ordinal of 0, meaning it is the first one in the set
func (cfg *Config) IsFirstPodInSet() bool {
	ordinal := getOrdinalFromHostname(cfg.Hostname)
	return ordinal == 0
}

// ShouldCloneFromBucket returns true if it's time to initialize from a bucket URL provided
func (cfg *Config) ShouldCloneFromBucket() bool {
	return !cfg.ExistsMySQLData && cfg.ServerID() == cfg.MyServerIDOffset && len(cfg.InitBucketURL) != 0
}

// XbstreamArgs returns a complete set of xbstream arguments.
func (cfg *Config) XbstreamArgs() []string {
	// xbstream --extract --directory=<mysql data dir> <extra args>
	xbstreamArgs := []string{"--extract", fmt.Sprintf("--directory=%s", dataDir)}
	return append(xbstreamArgs, cfg.XbstreamExtraArgs...)
}

// NewConfig returns a pointer to Config configured from environment variables
func NewConfig() *Config {
	var (
		err          error
		hbPass       string
		eData        bool
		offset       int
		customOffset string
	)

	if hbPass, err = rand.AlphaNumericString(10); err != nil {
		panic(err)
	}

	if eData, err = checkIfDataExists(); err != nil {
		panic(err)
	}

	offset = MysqlServerIDOffset
	customOffset = getEnvValue("MY_SERVER_ID_OFFSET")
	if len(customOffset) != 0 {
		if offset, err = strconv.Atoi(customOffset); err != nil {
			offset = MysqlServerIDOffset
		}
	}

	cfg := &Config{
		Hostname:    getEnvValue("HOSTNAME"),
		ClusterName: getEnvValue("MY_CLUSTER_NAME"),
		Namespace:   getEnvValue("MY_NAMESPACE"),
		ServiceName: getEnvValue("MY_SERVICE_NAME"),

		InitBucketURL: getEnvValue("INIT_BUCKET_URI"),

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

		ExistsMySQLData: eData,

		MyServerIDOffset: offset,

		XbstreamExtraArgs: strings.Fields(getEnvValue("XBSTREAM_EXTRA_ARGS")),
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

// retryLookupHost tries to figure out a host IPs with retries
func retryLookupHost(host string) ([]string, error) {
	// try to find the host IP
	IPs, err := net.LookupHost(host)
	for count := 0; count < 20 && err != nil; count++ {
		// retry looking up for ip because first query failed
		IPs, err = net.LookupHost(host)
		// sleep 100 milliseconds
		time.Sleep(100 * time.Millisecond)
	}

	return IPs, err
}

// nolint: gosec
func checkIfDataExists() (bool, error) {
	path := fmt.Sprintf("%s/mysql", dataDir)
	_, err := os.Open(path)

	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		log.Error(err, "failed to open file", "file", path)
		return false, err
	}

	return true, nil
}
