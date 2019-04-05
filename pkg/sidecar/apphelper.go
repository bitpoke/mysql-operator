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

package sidecar

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	// timeOut represents the number of tries to check mysql to be ready.
	timeOut = 60
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

// RunSidecarCommand is the main command, and represents the runtime helper that
// configures the mysql server
func RunSidecarCommand(cfg *Config, stop <-chan struct{}) error {
	log.Info("start initialization")

	// wait for mysql to be ready
	if err := waitForMysqlReady(cfg); err != nil {
		return fmt.Errorf("mysql is not ready, err: %s", err)
	}

	// deactivate super read only
	log.Info("temporary disable SUPER_READ_ONLY")
	if err := runQuery(cfg, "SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;"); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	// if it's slave set replication source (master host)
	log.V(1).Info("configure topology")
	if err := configTopology(cfg); err != nil {
		return err
	}

	// mark setup as complete by writing a row in config table
	log.V(1).Info("flag setup as complet")
	if err := markConfigurationDone(cfg); err != nil {
		return err
	}

	// if it's master node then make it writtable else make it read only
	log.V(1).Info("configure read only flag")
	if err := configReadOnly(cfg); err != nil {
		return err
	}

	log.V(1).Info("start http server")
	srv := newServer(cfg, stop)
	return srv.ListenAndServe()
}

func waitForMysqlReady(cfg *Config) error {
	log.V(1).Info("wait for mysql to be ready")

	for i := 0; i < timeOut; i++ {
		time.Sleep(1 * time.Second)
		if err := runQuery(cfg, "SELECT 1"); err == nil {
			break
		}
	}
	if err := runQuery(cfg, "SELECT 1"); err != nil {
		log.V(1).Info("mysql is not ready", "error", err)
		return err
	}

	log.V(1).Info("mysql is ready")
	return nil

}

func configReadOnly(cfg *Config) error {
	var query string
	if cfg.NodeRole() == MasterNode {
		query = "SET GLOBAL READ_ONLY = 0"
	} else {
		// TODO: make it super read only - but fix pt-heartbeat problem first
		query = "SET GLOBAL READ_ONLY = 1"
	}
	if err := runQuery(cfg, query); err != nil {
		return fmt.Errorf("failed to set read_only config, err: %s", err)
	}
	return nil
}

func configTopology(cfg *Config) error {
	if cfg.NodeRole() == SlaveNode {
		log.Info("setting up as slave")
		if shouldBootstrapNode() {
			log.Info("doing bootstrap")
			if gtid, err := readPurgedGTID(); err == nil {
				log.Info("RESET MASTER and setting GTID_PURGED", "gtid", gtid)
				if errQ := runQuery(cfg, "RESET MASTER; SET GLOBAL GTID_PURGED=?", gtid); errQ != nil {
					return errQ
				}
			} else {
				log.V(-1).Info("can't determine what GTID to purge", "error", err)
			}
		}

		// slave node
		query := `
		  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		    MASTER_HOST=?,
		    MASTER_USER=?,
		    MASTER_PASSWORD=?,
		    MASTER_CONNECT_RETRY=?;
		`
		if err := runQuery(cfg, query,
			cfg.MasterFQDN(), cfg.ReplicationUser, cfg.ReplicationPassword, connRetry,
		); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		query = "START SLAVE;"
		if err := runQuery(cfg, query); err != nil {
			log.Info("failed to start slave in the simple mode, trying a second method")
			// TODO: https://bugs.mysql.com/bug.php?id=83713
			query2 := `
			  reset slave;
			  start slave IO_THREAD;
			  stop slave IO_THREAD;
			  reset slave;
			  start slave;
			`
			if err := runQuery(cfg, query2); err != nil {
				return fmt.Errorf("failed to start slave node, err: %s", err)
			}
		}
	}

	return nil
}

func markConfigurationDone(cfg *Config) error {
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
	query = fmt.Sprintf(query, toolsDbName, toolsInitTableName)

	if err := runQuery(cfg, query, cfg.Hostname); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}

	return nil
}

// readPurgedGTID returns the GTID from xtrabackup_binlog_info file
func readPurgedGTID() (string, error) {
	file, err := os.Open(fmt.Sprintf("%s/xtrabackup_binlog_info", dataDir))
	if err != nil {
		return "", err
	}

	defer func() {
		if err1 := file.Close(); err1 != nil {
			log.Error(err1, "failed to close file")
		}
	}()

	return getGTIDFrom(file)
}

// getGTIDFrom parse the content from xtrabackup_binlog_info file passed as
// io.Reader and extracts the GTID.
func getGTIDFrom(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanWords)

	gtid := ""
	for i := 0; scanner.Scan(); i++ {
		if i >= 2 {
			gtid += scanner.Text()
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	} else if len(gtid) == 0 {
		return "", io.EOF
	}

	return gtid, nil
}
