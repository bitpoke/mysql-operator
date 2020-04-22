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
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

const (
	// connRetry specifies the interval between reconnection attempts to master
	connRetry = 1
)

// SQLInterface expose abstract operations that can be applied on a MySQL node
type SQLInterface interface {
	Wait(ctx context.Context) error
	DisableSuperReadOnly(ctx context.Context) (func(), error)
	ChangeMasterTo(ctx context.Context, host string, user string, pass string) error
	MarkConfigurationDone(ctx context.Context) error
	IsConfigured(ctx context.Context) (bool, error)
	SetPurgedGTID(ctx context.Context) error
	MarkSetGTIDPurged(ctx context.Context) error
	Host() string
}

type nodeSQLRunner struct {
	dsn  string
	host string

	enableBinLog bool
}

type sqlFactoryFunc func(dsn, host string) SQLInterface

func newNodeConn(dsn, host string) SQLInterface {
	return &nodeSQLRunner{
		dsn:          dsn,
		host:         host,
		enableBinLog: false,
	}
}

// Wait method pings MySQL until it's ready (runs SELECT 1;). The timeout should be set in ctx context.Context
func (r *nodeSQLRunner) Wait(ctx context.Context) error {
	log.V(1).Info("wait for mysql to be ready")

	for {
		select {
		case <-ctx.Done():
			// timeout expired
			return fmt.Errorf("timeout: mysql is not ready")
		case <-time.After(time.Second):
			if err := r.runQuery(ctx, "SELECT 1"); err == nil {
				return nil
			}
		}
	}
}

func (r *nodeSQLRunner) DisableSuperReadOnly(ctx context.Context) (func(), error) {
	enable := func() {
		err := r.runQuery(ctx, "SET GLOBAL SUPER_READ_ONLY = 1;")
		if err != nil {
			log.Error(err, "failed to set node super read only", "node", r.Host())
		}
	}
	return enable, r.runQuery(ctx, "SET GLOBAL READ_ONLY = 1; SET GLOBAL SUPER_READ_ONLY = 0;")
}

// ChangeMasterTo changes the master host and starts slave.
func (r *nodeSQLRunner) ChangeMasterTo(ctx context.Context, masterHost, user, pass string) error {
	// slave node
	query := `
      STOP SLAVE;
	  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		MASTER_HOST=?,
		MASTER_USER=?,
		MASTER_PASSWORD=?,
		MASTER_CONNECT_RETRY=?;
	`
	if err := r.runQuery(ctx, query,
		masterHost, user, pass, connRetry,
	); err != nil {
		return fmt.Errorf("failed to configure slave node, err: %s", err)
	}

	query = "START SLAVE;"
	if err := r.runQuery(ctx, query); err != nil {
		log.Info("failed to start slave in the simple mode, trying a second method", "host", r.Host())
		// TODO: https://bugs.mysql.com/bug.php?id=83713
		query2 := `
		  reset slave;
		  start slave IO_THREAD;
		  stop slave IO_THREAD;
		  reset slave;
		  start slave;
		`
		if err := r.runQuery(ctx, query2); err != nil {
			return fmt.Errorf("failed to start slave node, err: %s", err)
		}
	}
	return nil
}

// MarkConfigurationDone write in a MEMORY table value. The readiness probe checks for that value to exist to succeed.
func (r *nodeSQLRunner) MarkConfigurationDone(ctx context.Context) error {
	return r.writeStatusValue(ctx, "configured", "1")
}

// IsConfigured returns true if MySQL was configured, a key was set in the status table
func (r *nodeSQLRunner) IsConfigured(ctx context.Context) (bool, error) {
	val, err := r.readStatusValue(ctx, "configured")
	return val == "1", err
}

func (r *nodeSQLRunner) MarkSetGTIDPurged(ctx context.Context) error {
	return r.writeStatusValue(ctx, "set_gtid_purged", "skipped")
}

func (r *nodeSQLRunner) Host() string {
	return r.host
}

// runQuery executes a query
func (r *nodeSQLRunner) runQuery(ctx context.Context, q string, args ...interface{}) error {
	db, close, err := r.dbConn()
	if err != nil {
		return err
	}
	defer close()

	log.V(1).Info("running query", "query", q, "host", r.Host())

	if !r.enableBinLog {
		q = "SET @@SESSION.SQL_LOG_BIN = 0;\n" + q
	}
	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		return err
	}

	return nil
}

// readFromMysql executes the given query and loads the values into give variables
func (r *nodeSQLRunner) readFromMysql(ctx context.Context, query string, values ...interface{}) error {
	db, close, err := r.dbConn()
	if err != nil {
		return err
	}
	defer close()

	log.V(1).Info("running query", "query", query)
	err = db.QueryRowContext(ctx, query).Scan(values...)
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
			log.Error(cErr, "failed closing the database connection", "host", r.Host())
		}
	}

	return db, close, nil
}

func (r *nodeSQLRunner) SetPurgedGTID(ctx context.Context) error {
	var (
		err        error
		setGTID    string
		backupGTID string
	)

	// first check if the GTID was set before
	if setGTID, err = r.readStatusValue(ctx, "set_gtid_purged"); err != nil {
		return err
	} else if len(setGTID) != 0 {

		log.V(1).Info("GTID purged was already set", "host", r.Host(), "gtid_purged", setGTID)
		return nil
	}

	// check if there is any GTID to set
	if backupGTID, err = r.readStatusValue(ctx, "backup_gtid_purged"); err != nil {
		return err
	} else if len(backupGTID) == 0 {
		log.V(1).Info("no GTID to set", "host", r.Host())
		return nil
	}

	// GTID exists and should be set in a transaction
	// nolint: gosec
	query := fmt.Sprintf(`
	  SET @@SESSION.SQL_LOG_BIN = 0;
	  START TRANSACTION;
		SELECT value INTO @gtid FROM %[1]s.%[2]s WHERE name='%[3]s';
		RESET MASTER;
		SET @@GLOBAL.GTID_PURGED = @gtid;
		REPLACE INTO %[1]s.%[2]s VALUES ('%[4]s', @gtid);
	  COMMIT;
    `, constants.OperatorDbName, constants.OperatorStatusTableName, "backup_gtid_purged", "set_gtid_purged")

	if err := r.runQuery(ctx, query); err != nil {
		return err
	}

	return nil
}

// readStatusValue read from status table the value under the given key
func (r *nodeSQLRunner) readStatusValue(ctx context.Context, key string) (string, error) {
	// nolint: gosec
	qq := fmt.Sprintf("SELECT value FROM %s.%s WHERE name='%s'",
		constants.OperatorDbName, constants.OperatorStatusTableName, key)

	var value string
	if err := r.readFromMysql(ctx, qq, &value); err != nil {
		if err != sql.ErrNoRows {
			return "", err
		}
	}

	return value, nil
}

// writeStatusValue updates the value at the provided key
func (r *nodeSQLRunner) writeStatusValue(ctx context.Context, key, value string) error {
	// nolint: gosec
	query := fmt.Sprintf("REPLACE INTO %s.%s VALUES ('%s', '%s');",
		constants.OperatorDbName, constants.OperatorStatusTableName, key, value)

	if err := r.runQuery(ctx, query); err != nil {
		return err
	}

	return nil
}

// isMySQLError checks if a mysql error is of the given code.
// more information about mysql error codes can be found here:
// https://dev.mysql.com/doc/refman/8.0/en/server-error-reference.html
// nolint:unused,deadcode
func isMySQLError(err error, no int) bool {
	errStr := fmt.Sprintf("Error %d:", no)
	return strings.Contains(err.Error(), errStr)
}
