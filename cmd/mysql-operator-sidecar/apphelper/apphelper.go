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
	"database/sql"
	"fmt"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	tb "github.com/presslabs/mysql-operator/cmd/mysql-operator-sidecar/util"
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
	log.V(2).Info("temporary disable SUPER_READ_ONLY")
	if err := tb.RunQuery("SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;"); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	// update orchestrator user and password if orchestrator is configured
	log.V(2).Info("configure orchestrator credentials")
	if len(tb.GetOrcUser()) > 0 {
		if err := configureOrchestratorUser(); err != nil {
			return err
		}
	}

	// update replication user and password
	log.V(2).Info("configure replication credentials")
	if err := configureReplicationUser(); err != nil {
		return err
	}

	// update metrics exporter user and password
	log.V(2).Info("configure metrics exporter credentials")
	if err := configureExporterUser(); err != nil {
		return err
	}

	// if it's slave set replication source (master host)
	log.V(2).Info("configure topology")
	if err := configTopology(); err != nil {
		return err
	}

	// mark setup as complete by writing a row in config table
	log.V(2).Info("flag setup as complet")
	if err := markConfigurationDone(); err != nil {
		return err
	}

	// if it's master node then make it writtable else make it read only
	log.V(2).Info("configure read only flag")
	if err := configReadOnly(); err != nil {
		return err
	}

	log.V(2).Info("start http server")
	srv := newServer(stopCh)
	return srv.ListenAndServe()
}

func configureOrchestratorUser() error {
	query := `
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SUPER, PROCESS, REPLICATION SLAVE, REPLICATION CLIENT, RELOAD ON *.* TO '@user'@'%' IDENTIFIED BY '@password';
    GRANT SELECT ON @db_name.* TO '@user'@'%';
    GRANT SELECT ON mysql.slave_master_info TO '@user'@'%';
    `

	if err := tb.RunQuery(query, sql.Named("user", tb.GetOrcUser()),
		sql.Named("password", tb.GetOrcPass()),
		sql.Named("db_name", tb.ToolsDbName),
	); err != nil {
		return fmt.Errorf("failed to configure orchestrator (user/pass/access), err: %s", err)
	}

	return nil
}

func configureReplicationUser() error {
	query := `
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO '@user'@'%' IDENTIFIED BY '@password';
    `
	if err := tb.RunQuery(query, sql.Named("user", tb.GetReplUser()), sql.Named("password", tb.GetReplPass())); err != nil {
		return fmt.Errorf("failed to configure replication user: %s", err)
	}

	return nil
}

func configureExporterUser() error {
	query := `
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO '@user'@'127.0.0.1' IDENTIFIED BY '@password' WITH MAX_USER_CONNECTIONS 3;
    `

	if err := tb.RunQuery(query, sql.Named("user", tb.GetExporterUser()), sql.Named("password", tb.GetExporterPass())); err != nil {
		return fmt.Errorf("failed to metrics exporter user: %s", err)
	}

	return nil
}

func waitForMysqlReady() error {
	log.V(2).Info("wait for mysql to be ready")

	for i := 0; i < timeOut; i++ {
		time.Sleep(1 * time.Second)
		if err := tb.RunQuery("SELECT 1"); err == nil {
			break
		}
	}
	if err := tb.RunQuery("SELECT 1"); err != nil {
		log.V(2).Info("mysql is not ready", "error", err)
		return err
	}

	log.V(2).Info("mysql is ready")
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
		query := `
		  STOP SLAVE;
		  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
			MASTER_HOST='@host',
			MASTER_USER='@user',
			MASTER_PASSWORD='@password',
			MASTER_CONNECT_RETRY=@retry;
         `

		if err := tb.RunQuery(query, sql.Named("host", tb.GetMasterHost()),
			sql.Named("user", tb.GetReplUser()),
			sql.Named("password", tb.GetReplPass()),
			sql.Named("retry", connRetry),
		); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		query = "START SLAVE;"
		if err := tb.RunQuery(query); err != nil {
			log.Info("failed to start slave in the simple mode, trying a second method")
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

func markConfigurationDone() error {
	query := `
    SET @@SESSION.SQL_LOG_BIN = 0;
    BEGIN;
    CREATE DATABASE IF NOT EXISTS @db;
	CREATE TABLE IF NOT EXISTS @db.@table  (
	  name varchar(255) NOT NULL,
	  value varchar(255) NOT NULL,
	  inserted_at datetime NOT NULL
	) ENGINE=csv;

    INSERT INTO @db.@table VALUES ("init_completed", "@host", now());
    COMMIT;
    `

	if err := tb.RunQuery(query, sql.Named("db", tb.ToolsDbName),
		sql.Named("table", tb.ToolsInitTableName),
		sql.Named("host", tb.GetHostname()),
	); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}

	return nil
}
