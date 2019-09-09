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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/presslabs/mysql-operator/pkg/util/constants"

	"github.com/go-ini/ini"
)

// RunConfigCommand generates my.cnf, client.cnf and 10-dynamic.cnf files.
// nolint: gocyclo
func RunConfigCommand(cfg *Config) error {
	log.Info("configuring server", "host", cfg.Hostname)
	var err error

	if err = copyFile(mountConfigDir+"/my.cnf", configDir+"/my.cnf"); err != nil {
		return fmt.Errorf("copy file my.cnf: %s", err)
	}

	if err = os.Mkdir(confDPath, os.FileMode(0755)); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error mkdir %s/conf.d: %s", configDir, err)
		}
	}

	reportHost := cfg.FQDNForServer(cfg.ServerID())

	var identityCFG, initCFG, clientCFG, heartbeatCFG *ini.File

	// mysql server identity configs
	if identityCFG, err = getIdentityConfigs(cfg.ServerID(), reportHost); err != nil {
		return fmt.Errorf("failed to get dynamic configs: %s", err)
	}
	if err = identityCFG.SaveTo(path.Join(confDPath, "10-identity.cnf")); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	// write initialization sql file. This file is the init-file used by MySQL to configure itself
	var gtidPurged string
	gtidPurged, err = readPurgedGTID()
	if err != nil {
		// not a fatal error, log it and continue
		log.Info("error while reading PURGE GTID from xtrabackup info file", "error", err)
	}

	initFilePath := path.Join(confDPath, "operator-init.sql")
	if err = ioutil.WriteFile(initFilePath, initFileQuery(cfg, gtidPurged), 0644); err != nil {
		return fmt.Errorf("failed to write init-file: %s", err)
	}

	// mysql server utility user configs
	if initCFG, err = getInitFileConfigs(initFilePath); err != nil {
		return fmt.Errorf("failed to configure init file: %s", err)
	}
	if err = initCFG.SaveTo(path.Join(confDPath, "10-init-file.cnf")); err != nil {
		return fmt.Errorf("failed to configure init file: %s", err)
	}

	// mysql client connect credentials
	if clientCFG, err = getClientConfigs(cfg.OperatorUser, cfg.OperatorPassword); err != nil {
		return fmt.Errorf("failed to get client configs: %s", err)
	}

	if err = clientCFG.SaveTo(confClientPath); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	// mysql heartbeat connect credentials
	if heartbeatCFG, err = getClientConfigs(cfg.HeartBeatUser, cfg.HeartBeatPassword); err != nil {
		return fmt.Errorf("failed to get heartbeat configs: %s", err)
	}

	if err = heartbeatCFG.SaveTo(confHeartbeatPath); err != nil {
		return fmt.Errorf("failed to save heartbeat configs: %s", err)
	}

	return nil
}

func getClientConfigs(user, pass string) (*ini.File, error) {
	cfg := ini.Empty()
	// create client.cnf file
	client := cfg.Section("client")

	if _, err := client.NewKey("host", "127.0.0.1"); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("port", mysqlPort); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("user", user); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("password", pass); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getIdentityConfigs(id int, reportHost string) (*ini.File, error) {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	if _, err := mysqld.NewKey("server-id", strconv.Itoa(id)); err != nil {
		return nil, err
	}
	if _, err := mysqld.NewKey("report-host", reportHost); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getInitFileConfigs(filePath string) (*ini.File, error) {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	if _, err := mysqld.NewKey("init-file", filePath); err != nil {
		return nil, err
	}

	return cfg, nil
}

func initFileQuery(cfg *Config, gtidPurged string) []byte {
	queries := []string{
		"SET @@SESSION.SQL_LOG_BIN = 0",
	}

	// create operator database because GRANTS need it
	queries = append(queries, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", toolsDbName))

	// configure operator utility user
	queries = append(queries, createUserQuery(cfg.OperatorUser, cfg.OperatorPassword, "%",
		[]string{"SUPER", "SHOW DATABASES", "PROCESS", "RELOAD", "CREATE", "SELECT"}, "*.*",
		[]string{"ALL"}, fmt.Sprintf("%s.*", toolsDbName))...)

	// configure orchestrator user
	queries = append(queries, createUserQuery(cfg.OrchestratorUser, cfg.OrchestratorPassword, "%",
		[]string{"SUPER", "PROCESS", "REPLICATION SLAVE", "REPLICATION CLIENT", "RELOAD"}, "*.*",
		[]string{"SELECT"}, "mysql.slave_master_info",
		[]string{"SELECT", "CREATE"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName))...)

	// configure replication user
	queries = append(queries, createUserQuery(cfg.ReplicationUser, cfg.ReplicationPassword, "%",
		[]string{"SELECT", "PROCESS", "RELOAD", "LOCK TABLES", "REPLICATION CLIENT", "REPLICATION SLAVE"}, "*.*")...)

	// configure metrics exporter user
	queries = append(queries, createUserQuery(cfg.MetricsUser, cfg.MetricsPassword, "127.0.0.1",
		[]string{"SELECT", "PROCESS", "REPLICATION CLIENT"}, "*.*",
		[]string{"SELECT", "CREATE"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName))...)

	queries = append(queries, fmt.Sprintf("ALTER USER %s@'127.0.0.1' WITH MAX_USER_CONNECTIONS 3", cfg.MetricsUser))

	// configure heartbeat user
	// because of pt-heartbeat make sure not to have ALL or SUPER privileges:
	// https://github.com/percona/percona-toolkit/blob/e85ce15ef24bc4614b4d2f13792fa73583d68f8e/bin/pt-heartbeat#L6433
	queries = append(queries, createUserQuery(cfg.HeartBeatUser, cfg.HeartBeatPassword, "127.0.0.1",
		[]string{"CREATE", "SELECT", "DELETE", "UPDATE", "INSERT"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName),
		[]string{"REPLICATION CLIENT"}, "*.*")...)

	// create the status table used by the operator to configure or to mask MySQL node ready
	// CSV engine for this table can't be used because we use REPLACE statement that requires PRIMARY KEY or
	// UNIQUE KEY index
	// nolint: gosec
	queries = append(queries, fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %[1]s.%[2]s ("+
			"  name varchar(64) PRIMARY KEY,"+
			"  value varchar(512) NOT NULL\n)",
		constants.OperatorDbName, constants.OperatorStatusTableName))

	// mark node as not configured at startup, the operator will mark it configured
	// nolint: gosec
	queries = append(queries, fmt.Sprintf("REPLACE INTO %s.%s VALUES ('%s', '0')",
		constants.OperatorDbName, constants.OperatorStatusTableName, "configured"))

	if len(gtidPurged) != 0 {
		// if gtid is found in the backup then set it in the status table to be processed by the operator
		// nolint: gosec
		queries = append(queries, fmt.Sprintf(`REPLACE INTO %s.%s VALUES ('%s', '%s')`,
			constants.OperatorDbName, constants.OperatorStatusTableName, "backup_gtid_purged", gtidPurged))
	}

	// if just recently the node was initialized from a backup then a RESET SLAVE ALL query should be ran
	// to avoid not replicate from previous master.
	if cfg.ShouldCloneFromBucket() {
		queries = append(queries, "RESET SLAVE ALL")
	}

	return []byte(strings.Join(queries, ";\n") + ";\n")
}

func createUserQuery(name, pass, host string, rights ...interface{}) []string {
	user := fmt.Sprintf("%s@'%s'", name, host)

	queries := []string{
		fmt.Sprintf("DROP USER IF EXISTS %s", user),
		fmt.Sprintf("CREATE USER %s IDENTIFIED BY '%s'", user, pass),
	}

	if len(rights)%2 != 0 {
		panic("not a good number of parameters")
	}
	grants := []string{}
	for i := 0; i < len(rights); i += 2 {
		var (
			right []string
			on    string
			ok    bool
		)
		if right, ok = rights[i].([]string); !ok {
			panic("[right] not a good parameter")
		}
		if on, ok = rights[i+1].(string); !ok {
			panic("[on] not a good parameter")
		}
		grant := fmt.Sprintf("GRANT %s ON %s TO %s", strings.Join(right, ", "), on, user)
		grants = append(grants, grant)
	}

	return append(queries, grants...)
}
