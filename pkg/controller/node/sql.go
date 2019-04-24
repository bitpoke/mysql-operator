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
	"strings"
	"time"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

const (
	// mysqlReadyTries represents the number of tries with 1 second sleep between them to check if MySQL is ready
	mysqlReadyTries = 10
	// connRetry represents the number of tries to connect to master server
	connRetry = 10
)

type SQLInterface interface {
	Wait() error
	DisableSuperReadOnly() (func(), error)
	ChangeMasterTo(string, string, string) error
	MarkConfigurationDone() error
	SetPurgedGTID() error
	Host() string
}

type nodeSQLRunner struct {
	dsn  string
	host string

	enableBinLog bool
}

type newSQLInterface func(dsn, host string) SQLInterface

func newNodeConn(dsn, host string) SQLInterface {
	return &nodeSQLRunner{
		dsn:          dsn,
		host:         host,
		enableBinLog: false,
	}
}

func (r *nodeSQLRunner) Wait() error {
	log.V(1).Info("wait for mysql to be ready")

	for i := 0; i < mysqlReadyTries; i++ {
		if err := r.runQuery("SELECT 1"); err == nil {
			break
		}
		// wait a second
		time.Sleep(1 * time.Second)
	}
	if err := r.runQuery("SELECT 1"); err != nil {
		log.V(1).Info("mysql is not ready", "error", err)
		return err
	}

	log.V(1).Info("mysql is ready")
	return nil

}

func (r *nodeSQLRunner) DisableSuperReadOnly() (func(), error) {
	enable := func() {
		err := r.runQuery("SET GLOBAL SUPER_READ_ONLY = 0;")
		if err != nil {
			log.Error(err, "failed to set node super read only", "node", r.Host())
		}
	}
	return enable, r.runQuery("SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;")
}

// ChangeMasterTo changes the master host and starts slave.
func (r *nodeSQLRunner) ChangeMasterTo(masterHost, user, pass string) error {
	// slave node
	query := `
      STOP SLAVE;
	  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		MASTER_HOST=?,
		MASTER_USER=?,
		MASTER_PASSWORD=?,
		MASTER_CONNECT_RETRY=?;
	`
	if err := r.runQuery(query,
		masterHost, user, pass, connRetry,
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
    CREATE TABLE IF NOT EXISTS %s.%s (
		id int NOT NULL
    ) ENGINE=MEMORY;

    INSERT INTO %[1]s.%[2]s VALUES (1);
    `
	query = fmt.Sprintf(query, constants.OperatorDbName, "readiness")

	if err := r.runQuery(query); err != nil {
		return fmt.Errorf("failed to mark configuration done, err: %s", err)
	}
	return nil
}

func (r *nodeSQLRunner) Host() string {
	return r.host
}

// runQuery executes a query
func (r *nodeSQLRunner) runQuery(q string, args ...interface{}) error {
	db, close, err := r.dbConn()
	if err != nil {
		return err
	}
	defer close()

	log.V(1).Info("running query", "query", q)

	if !r.enableBinLog {
		q = "SET @@SESSION.SQL_LOG_BIN = 0;\n" + q
	}
	if _, err := db.Exec(q, args...); err != nil {
		return err
	}

	return nil
}

// readFromMysql executes the given query and loads the values into give variables
func (r *nodeSQLRunner) readFromMysql(query string, values ...interface{}) error {
	db, close, err := r.dbConn()
	if err != nil {
		return err
	}
	defer close()

	// nolint: gosec
	log.V(1).Info("running query", "query", query)
	row := db.QueryRow(query)
	if row == nil {
		return fmt.Errorf("no row found")
	}

	err = row.Scan(values...)
	if err != nil {
		return err
	}

	return nil
}

// dbConn this function returns a pointer to sql.DB. a function for closing the connection
// or an error if the MySQL can not be reached
func (r *nodeSQLRunner) dbConn() (*sql.DB, func(), error) {
	db, err := sql.Open("mysql", r.dsn)
	if err != nil {
		return nil, func() {}, err
	}
	close := func() {
		if cErr := db.Close(); cErr != nil {
			log.Error(cErr, "failed closing the database connection")
		}
	}

	return db, close, nil
}

func (r *nodeSQLRunner) SetPurgedGTID() error {
	// first check if the GTID should be set, if the table exists or if the GTID was set before (used)
	qq := fmt.Sprintf("SELECT used FROM %[1]s.%[2]s WHERE id=1",
		constants.OperatorDbName, constants.OperatorGtidsTableName)

	var used bool
	if err := r.readFromMysql(qq, &used); err != nil {
		// if it's a: "Table doesn't exist" error then GTID should not be set, it's a master case.
		if isMySQLError(err, 1146) {
			log.V(1).Info("GTID purged table does not exists", "host", r.Host())
			return nil
		}
		return err
	}

	if used {
		log.V(1).Info("GTID purged set", "host", r.Host())
		return nil
	}

	// GTID exists and should be set in a transaction
	query := fmt.Sprintf(`
      SET @@SESSION.SQL_LOG_BIN = 0;
      START TRANSACTION;
        SELECT gtid INTO @gtid FROM %[1]s.%[2]s WHERE id=1 AND used=false;
	    RESET MASTER;
	    SET @@GLOBAL.GTID_PURGED = @gtid;
	    REPLACE INTO %[1]s.%[2]s VALUES (1, @gtid, true);
      COMMIT;
    `, constants.OperatorDbName, constants.OperatorGtidsTableName)

	if err := r.runQuery(query); err != nil {
		return err
	}

	return nil
}

// isMySQLError checks if a mysql error is of the given code.
// more information about mysql error codes can be found here:
// https://dev.mysql.com/doc/refman/8.0/en/server-error-reference.html
func isMySQLError(err error, no int) bool {
	errStr := fmt.Sprintf("Error %d:", no)
	return strings.Contains(err.Error(), errStr)
}
