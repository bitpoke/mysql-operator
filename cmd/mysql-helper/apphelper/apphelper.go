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

package apphelper

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	tb "github.com/presslabs/mysql-operator/cmd/mysql-helper/util"
)

const (
	// timeOut represents the number of tries to check mysql to be ready.
	timeOut = 60
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

// RunRunCommand is the main command, and represents the runtime helper that
// configures the mysql server
func RunRunCommand(stopCh <-chan struct{}) error {
	glog.Infof("Starting initialization...")

	// wait for mysql to be ready
	if err := waitForMysqlReady(); err != nil {
		return fmt.Errorf("mysql is not ready, err: %s", err)
	}

	// deactivate super read only
	if err := tb.RunQuery("SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;"); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}
	glog.V(2).Info("Temporary disabled SUPER_READ_ONLY...")

	// update orchestrator user and password if orchestrator is configured
	if len(tb.GetOrcUser()) > 0 {
		if err := configureOrchestratorUser(); err != nil {
			return err
		}
	}
	glog.V(2).Info("Configured orchestrator user...")

	// update replication user and password
	if err := configureReplicationUser(); err != nil {
		return err
	}
	glog.V(2).Info("Configured replication user...")

	// update metrics exporter user and password
	if err := configureExporterUser(); err != nil {
		return err
	}
	glog.V(2).Info("Configured metrics exporter user...")

	// if it's slave set replication source (master host)
	if err := configTopology(); err != nil {
		return err
	}
	glog.V(2).Info("Configured topology...")

	if err := markConfigurationDone(); err != nil {
		return err
	}
	glog.V(2).Info("Flag setup as complete...")

	// if it's master node then make it writtable else make it read only
	if err := configReadOnly(); err != nil {
		return err
	}
	glog.V(2).Info("Configured read only flag...")

	srv := newServer(stopCh)
	glog.V(2).Info("Starting http server...")

	return srv.ListenAndServe()
}

// nolint: gas
func configureOrchestratorUser() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* TO '%[1]s'@'%%' IDENTIFIED BY '%[2]s';
    GRANT SELECT ON %[3]s.* TO '%[1]s'@'%%';
    GRANT SELECT ON mysql.slave_master_info TO '%[1]s'@'%%';
    `, tb.GetOrcUser(), tb.GetOrcPass(), tb.ToolsDbName)

	if err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure orchestrator (user/pass/access), err: %s", err)
	}

	return nil
}

func configureReplicationUser() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO '%s'@'%%' IDENTIFIED BY '%s';
    `, tb.GetReplUser(), tb.GetReplPass())
	if err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure replication user: %s", err)
	}

	return nil
}

func configureExporterUser() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'%%' IDENTIFIED BY '%s' WITH MAX_USER_CONNECTIONS 3;
    `, tb.GetExporterUser(), tb.GetExporterPass())
	if err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to metrics exporter user: %s", err)
	}

	return nil
}

func waitForMysqlReady() error {
	glog.V(2).Info("Wait for mysql to be ready.")

	for i := 0; i < timeOut; i++ {
		time.Sleep(1 * time.Second)
		if err := tb.RunQuery("SELECT 1"); err == nil {
			break
		}
	}
	if err := tb.RunQuery("SELECT 1"); err != nil {
		glog.V(2).Info("Mysql is not ready.")
		return err
	}
	glog.V(2).Info("Mysql is ready.")

	return nil

}

func configReadOnly() error {
	var query string
	if tb.NodeRole() == "master" {
		query = "SET GLOBAL READ_ONLY = 0"
	} else {
		query = "SET GLOBAL SUPER_READ_ONLY = 1"
	}
	if err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to set read_only config, err: %s", err)
	}
	return nil
}

func configTopology() error {
	if tb.NodeRole() == "slave" {
		// slave node
		query := fmt.Sprintf(`
			STOP SLAVE;
            CHANGE MASTER TO MASTER_AUTO_POSITION=1,
			  MASTER_HOST='%s',
			  MASTER_USER='%s',
			  MASTER_PASSWORD='%s',
			  MASTER_CONNECT_RETRY=%d;
         `, tb.GetMasterHost(), tb.GetReplUser(), tb.GetReplPass(), connRetry)

		if err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		query = `
        START SLAVE;
        `
		if err := tb.RunQuery(query); err != nil {
			glog.Warning("Failed to start slave simple, err: %s, try second method.")
			// TODO: https://bugs.mysql.com/bug.php?id=83713
			query2 := `
			reset slave;
			start slave IO_THREAD;
			stop slave IO_THREAD;
			reset slave;
			start slave;
            `
			if err := tb.RunQuery(query2); err != nil {
				return fmt.Errorf("failed to start slave node, err: %s", err)
			}
		}
	}

	return nil
}

// nolint: gas
func markConfigurationDone() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    BEGIN;
    CREATE DATABASE IF NOT EXISTS %[1]s;
	CREATE TABLE IF NOT EXISTS %[1]s.%[2]s  (
	  name varchar(255) NOT NULL,
	  value varchar(255) NOT NULL,
	  inserted_at datetime NOT NULL
	) ENGINE=csv;

    INSERT INTO %[1]s.%[2]s VALUES ("init_completed", "%s", now());
    COMMIT;
    `, tb.ToolsDbName, tb.ToolsInitTableName, tb.GetHostname())

	if err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}

	return nil
}
