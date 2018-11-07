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

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/mysql-operator/pkg/sidecar/util"
)

var log = logf.Log.WithName("sidecar.apphelper")

const (
	// timeOut represents the number of tries to check mysql to be ready.
	timeOut = 60
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

// RunRunCommand is the main command, and represents the runtime helper that
// configures the mysql server
func RunRunCommand(stopCh <-chan struct{}) error {
	log.Info("start initialization")

	// wait for mysql to be ready
	if err := waitForMysqlReady(); err != nil {
		return fmt.Errorf("mysql is not ready, err: %s", err)
	}

	// deactivate super read only
	log.Info("temporary disable SUPER_READ_ONLY")
	if err := util.RunQuery("SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;"); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	// update orchestrator user and password if orchestrator is configured
	log.V(1).Info("configure orchestrator credentials")
	if len(util.GetOrcUser()) > 0 {
		if err := configureOrchestratorUser(); err != nil {
			return err
		}
	}

	// update replication user and password
	log.V(1).Info("configure replication credentials")
	if err := configureReplicationUser(); err != nil {
		return err
	}

	// update metrics exporter user and password
	log.V(1).Info("configure metrics exporter credentials")
	if err := configureExporterUser(); err != nil {
		return err
	}

	// if it's slave set replication source (master host)
	log.V(1).Info("configure topology")
	if err := configTopology(); err != nil {
		return err
	}

	// mark setup as complete by writing a row in config table
	log.V(1).Info("flag setup as complet")
	if err := markConfigurationDone(); err != nil {
		return err
	}

	// if it's master node then make it writtable else make it read only
	log.V(1).Info("configure read only flag")
	if err := configReadOnly(); err != nil {
		return err
	}

	log.V(1).Info("start http server")
	srv := newServer(stopCh)
	return srv.ListenAndServe()
}

func configureOrchestratorUser() error {
	query := `
	  SET @@SESSION.SQL_LOG_BIN = 0;
	  GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* TO ?@'%%' IDENTIFIED BY ?;
	  GRANT SELECT ON %s.* TO ?@'%%';
	  GRANT SELECT ON mysql.slave_master_info TO ?@'%%';
	`

	// insert toolsDBName, it's not user input so it's safe. Can't use
	// placeholders for table names, see:
	// https://github.com/golang/go/issues/18478
	query = fmt.Sprintf(query, util.ToolsDbName)

	if err := util.RunQuery(query, util.GetOrcUser(), util.GetOrcPass(), util.GetOrcUser(), util.GetOrcUser()); err != nil {
		return fmt.Errorf("failed to configure orchestrator (user/pass/access), err: %s", err)
	}

	return nil
}

func configureReplicationUser() error {
	query := `
	  SET @@SESSION.SQL_LOG_BIN = 0;
	  GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO ?@'%' IDENTIFIED BY ?;
	`
	if err := util.RunQuery(query, util.GetReplUser(), util.GetReplPass()); err != nil {
		return fmt.Errorf("failed to configure replication user: %s", err)
	}

	return nil
}

func configureExporterUser() error {
	query := `
	  SET @@SESSION.SQL_LOG_BIN = 0;
	  GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO ?@'127.0.0.1' IDENTIFIED BY ? WITH MAX_USER_CONNECTIONS 3;
	`
	if err := util.RunQuery(query, util.GetExporterUser(), util.GetExporterPass()); err != nil {
		return fmt.Errorf("failed to metrics exporter user: %s", err)
	}

	return nil
}

func waitForMysqlReady() error {
	log.V(1).Info("wait for mysql to be ready")

	for i := 0; i < timeOut; i++ {
		time.Sleep(1 * time.Second)
		if err := util.RunQuery("SELECT 1"); err == nil {
			break
		}
	}
	if err := util.RunQuery("SELECT 1"); err != nil {
		log.V(1).Info("mysql is not ready", "error", err)
		return err
	}

	log.V(1).Info("mysql is ready")
	return nil

}

func configReadOnly() error {
	var query string
	if util.NodeRole() == "master" {
		query = "SET GLOBAL READ_ONLY = 0"
	} else {
		query = "SET GLOBAL SUPER_READ_ONLY = 1"
	}
	if err := util.RunQuery(query); err != nil {
		return fmt.Errorf("failed to set read_only config, err: %s", err)
	}
	return nil
}

func configTopology() error {
	if util.NodeRole() == "slave" {
		// slave node
		query := `
		  STOP SLAVE;
		  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		    MASTER_HOST=?,
		    MASTER_USER=?,
		    MASTER_PASSWORD=?,
		    MASTER_CONNECT_RETRY=?;
		`
		if err := util.RunQuery(query,
			util.GetMasterHost(), util.GetReplUser(), util.GetReplPass(), connRetry,
		); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		query = "START SLAVE;"
		if err := util.RunQuery(query); err != nil {
			log.Info("failed to start slave in the simple mode, trying a second method")
			// TODO: https://bugs.mysql.com/bug.php?id=83713
			query2 := `
			  reset slave;
			  start slave IO_THREAD;
			  stop slave IO_THREAD;
			  reset slave;
			  start slave;
			`
			if err := util.RunQuery(query2); err != nil {
				return fmt.Errorf("failed to start slave node, err: %s", err)
			}
		}
	}

	return nil
}

func markConfigurationDone() error {
	query := `
	  SET @@SESSION.SQL_LOG_BIN = 0;
	  BEGIN;
	  CREATE DATABASE IF NOT EXISTS %[1]s;
	      CREATE TABLE IF NOT EXISTS %[1]s.%[2]s  (
		name varchar(255) NOT NULL,
		value varchar(255) NOT NULL,
		inserted_at datetime NOT NULL
	      ) ENGINE=csv;

	  INSERT INTO %[1]s.%[2]s VALUES ("init_completed", "?", now());
	  COMMIT;
	`

	// insert tables and databases names. It's safe because is not user input.
	// see: https://github.com/golang/go/issues/18478
	query = fmt.Sprintf(query, util.ToolsDbName, util.ToolsInitTableName)

	if err := util.RunQuery(query, util.GetHostname()); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}

	return nil
}
