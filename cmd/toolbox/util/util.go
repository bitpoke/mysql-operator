package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"github.com/presslabs/titanium/pkg/mysqlcluster"
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
)

func GetHostname() string {
	return os.Getenv("HOSTNAME")
}

func NodeRole() string {
	// TODO: interogate ORC
	if getOrdinal() == 0 {
		return "master"
	}

	// TODO: remove this ...
	if strings.Contains(GetHostname(), "master") {
		return "master"
	} else {
		return "slave"
	}
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
	return getOrdinal()
}

// GetHostFor returns the host for given server id
func GetHostFor(id int) string {
	base := strings.Split(GetHostname(), "-")
	govSVC := os.Getenv("TITANIUM_HEADLESS_SERVICE")
	return fmt.Sprintf("%s-%d.%s", strings.Join(base[:len(base)-1], "-"), id, govSVC)
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

// GetMasterHost returns the master service name from env variable
// MASTER_SERVICE_NAME
func GetMasterHost() string {
	// TODO: interogate ORC for master
	return GetHostFor(0)
}
