package util

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/golang/glog"

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

	// MountConfigDir is the mounted configs that needs processing
	MountConfigDir = mysqlcluster.ConfMapVolumeMountPath

	// DataDir is the mysql data. /var/lib/mysql
	DataDir = mysqlcluster.DataVolumeMountPath

	// CheckDataFile represent the check file that marks cloning complete
	CheckDataFile = ConfigDir + "/titanium_chk"

	// UtilityUser is the name of the percona utility user.
	UtilityUser = "utility"

	// OrcTopologyDir contains the path where the secret with orc credentails is
	// mounted.
	OrcTopologyDir = mysqlcluster.OrcTopologyDir
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
	base := strings.Split(GetHostname(), "-")
	govSVC := os.Getenv("TITANIUM_HEADLESS_SERVICE")
	return fmt.Sprintf("%s-%d.%s", strings.Join(base[:len(base)-1], "-"), id-100, govSVC)
}

// GetReplUser returns the replication user name from env variable
// TITANIUM_REPLICATION_USER
func GetReplUser() string {
	user := os.Getenv("TITANIUM_REPLICATION_USER")
	if len(user) == 0 {
		glog.Warning("TITANIUM_REPLICATION_USER is not set!")
	}

	return user
}

// GetReplPass returns the replication password from env variable
// TITANIUM_REPLICATION_PASSWORD
func GetReplPass() string {
	pass := os.Getenv("TITANIUM_REPLICATION_PASSWORD")
	if len(pass) == 0 {
		glog.Warning("TITANIUM_REPLICATION_PASSWORD is not set!")
	}

	return pass
}

// GetInitBucket returns the bucket uri from env variable
// TITANIUM_INIT_BUCKET_URI
func GetInitBucket() string {
	uri := os.Getenv("TITANIUM_INIT_BUCKET_URI")
	if len(uri) == 0 {
		glog.Warning("TIANIUM_INIT_BUCKET_URI is not set!")
	}

	return uri
}

// GetMasterHost returns the master host
func GetMasterHost() string {
	orcUri := getOrcUri()
	if len(orcUri) == 0 {
		glog.Warning("Orchestrator is not used!")
		return GetHostFor(100)
	}

	client := orc.NewFromUri(orcUri)
	host, _, err := client.Master(GetClusterName())
	if err != nil {
		glog.Errorf("Failed to connect to orc for finding master, err: %s."+
			" Fallback to determine master by its id.", err)
		return GetHostFor(100)
	}

	return host
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
	glog.V(2).Info("Creating orchestrator user and password...")
	query := fmt.Sprintf(
		"GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* "+
			"TO '%s'@'%%' IDENTIFIED BY '%s'", GetOrcUser(), GetOrcPass(),
	)
	if _, err := RunQuery(query); err != nil {
		return fmt.Errorf("failed to create orc user, err: %s", err)
	}

	query = fmt.Sprintf("GRANT SELECT ON meta.* TO '%s'@'%%'", GetOrcUser())
	if _, err := RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure orc user, err: %s", err)
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
	uri := os.Getenv("TITANIUM_ORC_URI")
	if len(uri) == 0 {
		glog.Warning("TIANIUM_ORC_URI is not set!")
	}

	return uri
}
