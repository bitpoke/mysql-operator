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

package apptitanium

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
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

func RunRunCommand(stopCh <-chan struct{}) error {
	glog.Infof("Starting initialization...")

	// wait for mysql to be ready
	if err := waitForMysqlReady(); err != nil {
		return fmt.Errorf("mysql is not ready, err: %s", err)
	}

	// deactivate read only
	if _, err := tb.RunQuery("SET GLOBAL READ_ONLY = 0"); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	// update orchestrator user and password if orchestrator is configured
	if len(tb.GetOrcUser()) > 0 {
		if err := tb.UpdateOrcUserPass(); err != nil {
			return err
		}
	}

	// update replication user and password
	if err := configureReplicationUser(); err != nil {
		return err
	}

	// if it's slave set replication source (master host)
	if err := configTopology(); err != nil {
		return err
	}

	if err := markConfigurationDone(); err != nil {
		return err
	}

	// if it's master node then make it writtable else make it read only
	if err := configReadOnly(); err != nil {
		return err
	}

	// start http server for readiness probe
	// here the server is ready to accept traffic
	httpServer(stopCh)

	// now serve backups
	return startServeBackups()
}

func configureReplicationUser() error {
	query := fmt.Sprintf(`
    SET @@SESSION.SQL_LOG_BIN = 0;
    GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO '%s'@'%%' IDENTIFIED BY '%s';
    `, tb.GetReplUser(), tb.GetReplPass())
	if _, err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to configure master node, err: %s", err)
	}

	return nil
}

func startServeBackups() error {
	glog.Infof("Serve backups command.")

	xtrabackup_cmd := []string{"xtrabackup", "--backup", "--slave-info", "--stream=xbstream",
		"--host=127.0.0.1", fmt.Sprintf("--user=%s", tb.GetReplUser()),
		fmt.Sprintf("--password=%s", tb.GetReplPass())}

	ncat := exec.Command("ncat", "--listen", "--keep-open", "--send-only", "--max-conns=1",
		tb.BackupPort, "-c", strings.Join(xtrabackup_cmd, " "))

	return ncat.Run()

}

func httpServer(stop <-chan struct{}) {
	mux := http.NewServeMux()

	// Add health endpoint
	mux.HandleFunc(tb.TitaniumProbePath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", tb.TitaniumProbePort),
		Handler: mux,
	}

	// Shutdown gracefully the http server
	go func() {
		<-stop // wait for stop signal
		if err := srv.Shutdown(context.Background()); err != nil {
			glog.Errorf("Failed to stop probe server, err: %s", err)
		}
	}()
	go func() {
		glog.Fatal(srv.ListenAndServe())
	}()
}

func waitForMysqlReady() error {
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

	return nil

}

func configReadOnly() error {
	var query string
	if tb.NodeRole() == "master" {
		query = "SET GLOBAL READ_ONLY = 0"
	} else {
		query = "SET GLOBAL SUPER_READ_ONLY = 1"
	}
	if _, err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("failed to set read_only config, err: %s", err)
	}
	return nil
}

func configTopology() error {
	if tb.NodeRole() == "slave" {
		// slave node
		// TODO: check why need stop slave; here
		query := fmt.Sprintf(`
            STOP SLAVE;
            CHANGE MASTER TO MASTER_AUTO_POSITION=1,
			  MASTER_HOST='%s',
			  MASTER_USER='%s',
			  MASTER_PASSWORD='%s',
			  MASTER_CONNECT_RETRY=%d;
         `, tb.GetMasterHost(), tb.GetReplUser(), tb.GetReplPass(), connRetry)

		if _, err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}

		// TODO: fix this issue
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

	if _, err := tb.RunQuery(query); err != nil {
		return fmt.Errorf("to mark configuration done, err: %s", err)
	}

	return nil
}
