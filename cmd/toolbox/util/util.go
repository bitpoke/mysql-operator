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
	BackupPort = strconv.Itoa(mysqlcluster.TitaniumXtrabackupPort)
)

func GetHostname() string {
	return os.Getenv("HOSTNAME")
}

func NodeRole() string {
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
	o := getOrdinal()

	var id int
	if NodeRole() == "master" {
		id = o
	} else {
		id = o + 100
	}

	return id
}

// GetHostFor returns the host for given server id
func GetHostFor(id int) string {
	base := strings.Split(GetHostname(), "-")
	govSVC := os.Getenv("TITANIUM_HEADLESS_SERVICE")
	return fmt.Sprintf("%s-%d.%s", base, id, govSVC)
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

// GetMasterService returns the master service name from env variable
// MASTER_SERVICE_NAME
func GetMasterService() string {
	s := os.Getenv("MASTER_SERVICE_NAME")
	if len(s) == 0 {
		glog.Warning("MASTER_SERVICE_NAME is not set!")
	}

	return s
}
