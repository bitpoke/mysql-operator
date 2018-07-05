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

package util

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-ini/ini"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/mysqlcluster"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

var (
	// BackupPort is the port on which xtrabackup expose backups, 3306
	BackupPort = strconv.Itoa(mysqlcluster.HelperXtrabackupPort)

	// MysqlPort represents port on wich mysql works
	MysqlPort = strconv.Itoa(mysqlcluster.MysqlPort)

	// ConfigDir is the mysql configs path, /etc/mysql
	ConfigDir = mysqlcluster.ConfVolumeMountPath

	// ConfDPath is /etc/mysql/conf.d
	ConfDPath = mysqlcluster.ConfDPath

	// MountConfigDir is the mounted configs that needs processing
	MountConfigDir = mysqlcluster.ConfMapVolumeMountPath

	// DataDir is the mysql data. /var/lib/mysql
	DataDir = mysqlcluster.DataVolumeMountPath

	// ToolsDbName is the name of the tools table
	ToolsDbName = mysqlcluster.HelperDbName
	// ToolsTableName is the name of the init table
	ToolsInitTableName = "init"

	// UtilityUser is the name of the percona utility user.
	UtilityUser = "sys_utility_helper"

	// OrcTopologyDir contains the path where the secret with orc credentails is
	// mounted.
	OrcTopologyDir = mysqlcluster.OrcTopologyDir

	NameOfStatefulSet = api.StatefulSet

	// http server config
	ServerPort           = mysqlcluster.HelperServerPort
	ServerProbeEndpoint  = mysqlcluster.HelperServerProbePath
	ServerBackupEndpoint = "/xbackup"
)

const (
	// rcloneConfigFile represents the path to the file that contains rclon
	// configs. This path should be the same as defined in docker entrypoint
	// script from mysql-helper/docker-entrypoint.sh. /etc/rclone.conf
	RcloneConfigFile = "/etc/rclone.conf"
)

func GetHostname() string {
	return os.Getenv("HOSTNAME")
}

func GetClusterName() string {
	return getEnvValue("MY_CLUSTER_NAME")
}

func GetNamespace() string {
	return getEnvValue("MY_NAMESPACE")
}

func GetServiceName() string {
	return getEnvValue("MY_SERVICE_NAME")
}

func NodeRole() string {
	if GetMasterHost() == GetHostFor(GetServerId()) {
		return "master"
	}
	return "slave"
}

func getOrdinal() int {
	hn := GetHostname()
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

func GetServerId() int {
	return 100 + getOrdinal()
}

// GetHostFor returns the host for given server id
func GetHostFor(id int) string {
	base := api.GetNameForResource(NameOfStatefulSet, GetClusterName())
	govSVC := GetServiceName()
	namespace := GetNamespace()
	return fmt.Sprintf("%s-%d.%s.%s", base, id-100, govSVC, namespace)
}

func getEnvValue(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		glog.Warning(fmt.Sprintf("%s is not set!", key))
	}

	return value
}

// GetReplUser returns the replication user name from env variable
// MYSQL_REPLICATION_USER
func GetReplUser() string {
	return getEnvValue("MYSQL_REPLICATION_USER")
}

// GetReplPass returns the replication password from env variable
// MYSQL_REPLICATION_PASSWORD
func GetReplPass() string {
	return getEnvValue("MYSQL_REPLICATION_PASSWORD")
}

// GetExporterUser returns the replication user name from env variable
// MYSQL_METRICS_EXPORTER_USER
func GetExporterUser() string {
	return getEnvValue("MYSQL_METRICS_EXPORTER_USER")
}

// GetExporterPass returns the replication password from env variable
// MYSQL_METRICS_EXPORTER_PASSWORD
func GetExporterPass() string {
	return getEnvValue("MYSQL_METRICS_EXPORTER_PASSWORD")
}

// GetInitBucket returns the bucket uri from env variable
// INIT_BUCKET_URI
func GetInitBucket() string {
	return getEnvValue("INIT_BUCKET_URI")
}

// GetBackupAccessUser returns the basic auth credentials to access backup
func GetBackupUser() string {
	return getEnvValue("MYSQL_BACKUP_USER")
}

// GetBackupAccessUser returns the basic auth credentials to access backup
func GetBackupPass() string {
	return getEnvValue("MYSQL_BACKUP_PASSWORD")
}

// GetMasterHost returns the master host
func GetMasterHost() string {
	orcUri := getOrcUri()
	if len(orcUri) == 0 {
		glog.Warning("Orchestrator is not used!")
		return GetHostFor(100)
	}

	fqClusterName := fmt.Sprintf("%s.%s", GetClusterName(), GetNamespace())

	client := orc.NewFromUri(orcUri)
	inst, err := client.Master(fqClusterName)
	if err != nil {
		glog.Errorf("Failed to connect to orc for finding master, err: %s."+
			" Fallback to determine master by its id.", err)
		return GetHostFor(100)
	}

	return inst.Key.Hostname
}

// GetOrcTopologyUser returns the orchestrator topology user from env variable
// MYSQL_ORC_TOPOLOGY_USER
func GetOrcUser() string {
	return getEnvValue("MYSQL_ORC_TOPOLOGY_USER")
}

// GetOrcTopologyPass returns the orchestrator topology passowrd from env variable
// MYSQL_ORC_TOPOLOGY_PASSWORD
func GetOrcPass() string {
	return getEnvValue("MYSQL_ORC_TOPOLOGY_PASSWORD")
}

func readFileContent(fileName string) string {
	f, err := os.Open(fileName)
	if err != nil {
		glog.Warningf("%s is not set, or can't be readed, see err: %s", fileName, err)
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		glog.Warningf("can't scan file: %s", fileName)
		return ""
	}

	return scanner.Text()
}

func GetMySQLConnectionString() (dsn string, err error) {
	cnfPath := path.Join(ConfigDir, "client.cnf")
	cfg, err := ini.Load(cnfPath)
	if err != nil {
		return "", fmt.Errorf("Could not open %s: %s", cnfPath, err)
	}
	client := cfg.Section("client")
	host := client.Key("host").String()
	user := client.Key("user").String()
	passowrd := client.Key("password").String()
	port, err := client.Key("port").Int()
	if err != nil {
		return "", fmt.Errorf("Invalid port in %s: %s", cnfPath, err)
	}
	dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true",
		user, passowrd, host, port,
	)
	return
}

func RunQuery(q string) (err error) {
	dsn, err := GetMySQLConnectionString()
	if err != nil {
		glog.Warningf("Could not get mysql connection dsn: %s", err)
		return
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		glog.Warningf("Could not open mysql connection: %s", err)
		return
	}

	glog.V(4).Infof("Running query: %s", q)
	if _, err := db.Query(q); err != nil {
		glog.Warningf("Could not query mysql: %s", err)
		return err
	}

	return
}

func getOrcUri() string {
	return getEnvValue("ORCHESTRATOR_URI")
}

// CopyFile the src file to dst. Any existing file will be overwritten and will not
// copy file attributes.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func MaxClients(h http.Handler, n int) http.Handler {
	sema := make(chan struct{}, n)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sema <- struct{}{}
		defer func() { <-sema }()

		h.ServeHTTP(w, r)
	})
}

func RequestABackup(host, endpoint string) (io.Reader, error) {
	glog.Infof("Initiate a backup from: %s endpoint: %s", host, endpoint)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d%s", host, ServerPort, endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to create request: %s", err)
	}

	// set authentification user and password
	req.SetBasicAuth(GetBackupUser(), GetBackupPass())

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		status := "unknown"
		if resp != nil {
			status = resp.Status
		}
		return nil, fmt.Errorf("fail to get backup: %s, code: %s", err, status)
	}

	return resp.Body, nil
}
