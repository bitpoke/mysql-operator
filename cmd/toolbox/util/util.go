package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	"github.com/presslabs/titanium/pkg/mysqlcluster"
	orc "github.com/presslabs/titanium/pkg/util/orchestrator"
)

var (
	// BackupPort is the port on which xtrabackup expose backups, 3306
	BackupPort = strconv.Itoa(mysqlcluster.TitaniumXtrabackupPort)

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
	ToolsDbName = "tools"
	// ToolsTableName is the name of the init table
	ToolsInitTableName = "init"

	// UtilityUser is the name of the percona utility user.
	UtilityUser = "sys_titanium"

	// OrcTopologyDir contains the path where the secret with orc credentails is
	// mounted.
	OrcTopologyDir = mysqlcluster.OrcTopologyDir

	NameOfStatefulSet = api.StatefulSet

	TitaniumProbePath = mysqlcluster.TitaniumProbePath
	TitaniumProbePort = mysqlcluster.TitaniumProbePort
)

const (
	// rcloneConfigFile represents the path to the file that contains rclon
	// configs. This path should be the same as defined in docker entrypoint
	// script from toolbox/docker-entrypoint.sh. /etc/rclone.conf
	RcloneConfigFile = "/etc/rclone.conf"
)

func GetHostname() string {
	return os.Getenv("HOSTNAME")
}

func GetClusterName() string {
	hn := GetHostname()
	l := strings.Split(hn, "-")

	return l[0]
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
	base := fmt.Sprintf("%s-%s", GetClusterName(), NameOfStatefulSet)
	govSVC := getEnvValue("TITANIUM_HEADLESS_SERVICE")
	return fmt.Sprintf("%s-%d.%s", base, id-100, govSVC)
}

func getEnvValue(key string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		glog.Warning(fmt.Sprintf("%s is not set!", key))
	}

	return value
}

// GetReplUser returns the replication user name from env variable
// TITANIUM_REPLICATION_USER
func GetReplUser() string {
	return getEnvValue("TITANIUM_REPLICATION_USER")
}

// GetReplPass returns the replication password from env variable
// TITANIUM_REPLICATION_PASSWORD
func GetReplPass() string {
	return getEnvValue("TITANIUM_REPLICATION_PASSWORD")
}

// GetExporterUser returns the replication user name from env variable
// TITANIUM_EXPORTER_USER
func GetExporterUser() string {
	return getEnvValue("TITANIUM_EXPORTER_USER")
}

// GetExporterPass returns the replication password from env variable
// TITANIUM_EXPORTER_PASSWORD
func GetExporterPass() string {
	return getEnvValue("TITANIUM_EXPORTER_PASSWORD")
}

// GetInitBucket returns the bucket uri from env variable
// TITANIUM_INIT_BUCKET_URI
func GetInitBucket() string {
	return getEnvValue("TITANIUM_INIT_BUCKET_URI")
}

// GetMasterHost returns the master host
func GetMasterHost() string {
	orcUri := getOrcUri()
	if len(orcUri) == 0 {
		glog.Warning("Orchestrator is not used!")
		return GetHostFor(100)
	}

	client := orc.NewFromUri(orcUri)
	inst, err := client.Master(GetClusterName())
	if err != nil {
		glog.Errorf("Failed to connect to orc for finding master, err: %s."+
			" Fallback to determine master by its id.", err)
		return GetHostFor(100)
	}

	return inst.Key.Hostname
}

// GetOrcTopologyUser returns the orchestrator topology user. It is readed from
// /var/run/orc-topology/TOPOLOGY_USER
func GetOrcUser() string {
	return readFileContent(OrcTopologyDir + "/TOPOLOGY_USER")
}

// GetOrcTopologyPass returns the orchestrator topology user. It is readed from
// /var/run/orc-topology/TOPOLOGY_PASSWORD
func GetOrcPass() string {
	return readFileContent(OrcTopologyDir + "/TOPOLOGY_PASSWORD")
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

func UpdateOrcUserPass() error {
	glog.V(2).Info("Creating orchestrator user, password and privileges...")
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* TO '%[1]s'@'%%' IDENTIFIED BY '%[2]s';
    GRANT SELECT ON meta.* TO '%[1]s'@'%%';
    GRANT SELECT ON mysql.slave_master_info TO '%[1]s'@'%%';
    `, GetOrcUser(), GetOrcPass())

	if _, err := RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure orchestrator (user/pass/access), err: %s", err)
	}
	glog.V(2).Info("Orchestrator user configured!")

	return nil
}

func RunQuery(q string) (string, error) {
	glog.V(3).Infof("QUERY: %s", q)

	mysql := exec.Command("mysql",
		fmt.Sprintf("--defaults-file=%s/%s", ConfigDir, "client.cnf"),
	)

	// write query through pipe to mysql
	rq := strings.NewReader(q)
	mysql.Stdin = rq
	var bufOUT, bufERR bytes.Buffer
	mysql.Stdout = &bufOUT
	mysql.Stderr = &bufERR

	if err := mysql.Run(); err != nil {
		glog.Errorf("Failed to run query, err: %s", err)
		glog.V(2).Infof("Mysql STDOUT: %s, STDERR: %s", bufOUT.String(), bufERR.String())
		return "", err
	}

	glog.V(2).Infof("Mysql STDOUT: %s, STDERR: %s", bufOUT.String(), bufERR.String())
	glog.V(3).Infof("Mysql output for query %s is: %s", q, bufOUT.String())

	return bufOUT.String(), nil
}

func getOrcUri() string {
	return getEnvValue("TITANIUM_ORC_URI")
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
