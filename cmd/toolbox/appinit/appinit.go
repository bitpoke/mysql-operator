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

package appinit

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

const (
	// timeOut represents the number of tries to check mysql to be ready.
	timeOut = 60
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

func RunInitCommand(stopCh <-chan struct{}) error {
	glog.Infof("Starting initialization...")

	glog.V(2).Info("Wait for mysql to be ready.")

	for i := 0; i < timeOut; i++ {
		time.Sleep(1 * time.Second)
		if _, err := tb.RunQuery("SELECT 1"); err == nil {
			break
		}
	}
	if _, err := tb.RunQuery("SELECT 1"); err != nil {
		glog.V(2).Info("Mysql is not ready.")
		return err
	}
	glog.V(2).Info("Mysql is ready.")

	if len(tb.GetOrcUser()) > 0 {
		if err := tb.UpdateOrcUserPass(); err != nil {
			return err
		}
	}

	if err := configureReplicationUser(); err != nil {
		return err
	}

	var query string
	if tb.NodeRole() == "master" {
		query = "SET GLOBAL READ_ONLY = 0"
	} else {
		query = "SET GLOBAL SUPER_READ_ONLY = 1"
	}
	if _, err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to set read_only config, err: %s", err)
	}

	if tb.NodeRole() == "slave" {
		// slave node
		query := fmt.Sprintf(
			"CHANGE MASTER TO MASTER_AUTO_POSITION=1,"+
				"MASTER_HOST='%s',"+
				"MASTER_USER='%s',"+
				"MASTER_PASSWORD='%s',"+
				"MASTER_CONNECT_RETRY=%d",
			tb.GetMasterHost(), tb.GetReplUser(), tb.GetReplPass(), connRetry)

		if _, err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		// https://bugs.mysql.com/bug.php?id=83713
		query = `
        RESET SLAVE;
        START SLAVE IO_THREAD;
        STOP SLAVE IO_THREAD;
        RESET SLAVE;
        START SLAVE;
        `
		if _, err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to start slave node, err: %s", err)
		}
	}
	return nil
}

func configureReplicationUser() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    SET GLOBAL READ_ONLY = 0;
    GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO '%s'@'%%' IDENTIFIED BY '%s';
    `, tb.GetReplUser(), tb.GetReplPass())
	if _, err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	return nil
}
