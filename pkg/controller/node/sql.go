/*
Copyright 2019 Pressinfra SRL

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

package node

import (
	"database/sql"
	"fmt"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
	"time"
)

const (
	// mysqlReadyTries represents the number of tries with 1 second sleep between them to check if MySQL is ready
	mysqlReadyTries = 20
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

type nodeSQLRunner struct {
	dsn  string
	host string

	replicationUser     string
	replicationPassword string
}

func newNodeConn(dsn, host, repU, repP string) *nodeSQLRunner {
	return &nodeSQLRunner{
		dsn:                 dsn,
		host:                host,
		replicationUser:     repU,
		replicationPassword: repP,
	}
}

func (r *nodeSQLRunner) Wait() error {
	log.V(1).Info("wait for mysql to be ready")

	for i := 0; i < mysqlReadyTries; i++ {
		time.Sleep(1 * time.Second)
		if err := r.runQuery("SELECT 1"); err == nil {
			break
		}
	}
	if err := r.runQuery("SELECT 1"); err != nil {
		log.V(1).Info("mysql is not ready", "error", err)
		return err
	}

	log.V(1).Info("mysql is ready")
	return nil

}

// runQuery executes a query
func (r *nodeSQLRunner) runQuery(q string, args ...interface{}) error {
	db, err := sql.Open("mysql", r.dsn)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			log.Error(cErr, "failed closing the database connection")
		}
	}()

	log.V(1).Info("running query", "query", q)
	if _, err := db.Exec(q, args...); err != nil {
		return err
	}

	return nil
}

func (r *nodeSQLRunner) SetReadOnly() error {
	return r.runQuery("SET GLOBAL SUPER_READ_ONLY = 1")
}

// TODO: if not used remove it
func (r *nodeSQLRunner) SetWritable() error {
	return r.runQuery("SET GLOBAL SUPER_READ = 0")
}

func (r *nodeSQLRunner) ConfigureSlaveNode(masterHost string) error {
	// slave node
	query := `
	  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		MASTER_HOST=?,
		MASTER_USER=?,
		MASTER_PASSWORD=?,
		MASTER_CONNECT_RETRY=?;
	`
	if err := r.runQuery(query,
		masterHost, r.replicationUser, r.replicationPassword, connRetry,
	); err != nil {
		return fmt.Errorf("failed to configure slave node, err: %s", err)
	}

	query = "START SLAVE;"
	if err := r.runQuery(query); err != nil {
		log.Info("failed to start slave in the simple mode, trying a second method")
		// TODO: https://bugs.mysql.com/bug.php?id=83713
		query2 := `
		  reset slave;
		  start slave IO_THREAD;
		  stop slave IO_THREAD;
		  reset slave;
		  start slave;
		`
		if err := r.runQuery(query2); err != nil {
			return fmt.Errorf("failed to start slave node, err: %s", err)
		}
	}
	return nil
}

func (r *nodeSQLRunner) MarkConfigurationDone() error {
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
	query = fmt.Sprintf(query, constants.OperatorDbName, constants.OperatorInitTableName)

	if err := r.runQuery(query, r.host); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}

	return nil
}

func (r *nodeSQLRunner) Host() string {
	return r.host
}
