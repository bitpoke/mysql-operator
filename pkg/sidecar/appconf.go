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

	"github.com/go-ini/ini"
)

// RunConfigCommand generates my.cnf, client.cnf and 10-dynamic.cnf files.
// nolint: gocyclo
func RunConfigCommand(cfg *Config) error {
	log.Info("configuring server", "host", cfg.Hostname, "role", cfg.NodeRole())
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

	var identityCFG, initCFG, clientCFG *ini.File

	// mysql server identity configs
	if identityCFG, err = getIdentityConfigs(cfg.ServerID(), reportHost); err != nil {
		return fmt.Errorf("failed to get dynamic configs: %s", err)
	}
	if err = identityCFG.SaveTo(path.Join(confDPath, "10-identity.cnf")); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	// make init-file
	initFilePath := path.Join(confDPath, "operator-init.sql")
	if err = ioutil.WriteFile(initFilePath, initFileQuery(cfg), 0644); err != nil {
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

func initFileQuery(cfg *Config) []byte {
	queries := []string{
		"SET @@SESSION.SQL_LOG_BIN = 0;",
	}

	// configure operator utility user
	queries = append(queries, createUserQuery(cfg.OperatorUser, cfg.OperatorPassword, "%",
		[]string{"SUPER", "SHOW DATABASES", "PROCESS", "RELOAD", "CREATE", "SELECT"}, "*.*",
		//[]string{"ALL"}, "*.*", // TODO: remove this before commit
		[]string{"ALL"}, fmt.Sprintf("%s.*", toolsDbName)))

	// configure orchestrator user
	queries = append(queries, createUserQuery(cfg.OrchestratorUser, cfg.OrchestratorPassword, "%",
		[]string{"SUPER", "PROCESS", "REPLICATION SLAVE", "REPLICATION CLIENT", "RELOAD"}, "*.*",
		[]string{"SELECT"}, "mysql.slave_master_info"))

	// configure replication user
	queries = append(queries, createUserQuery(cfg.ReplicationUser, cfg.ReplicationPassword, "%",
		[]string{"SELECT", "PROCESS", "RELOAD", "LOCK TABLES", "REPLICATION CLIENT", "REPLICATION SLAVE"}, "*.*"))

	// configure metrics exporter user
	queries = append(queries, createUserQuery(cfg.MetricsUser, cfg.MetricsPassword, "127.0.0.1",
		[]string{"SELECT", "PROCESS", "REPLICATION CLIENT"}, "*.*"))

	return []byte(strings.Join(queries, "\n"))
}

func createUserQuery(name, pass, host string, rights ...interface{}) string {
	user := fmt.Sprintf("%s@'%s'", name, host)

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
		grant := fmt.Sprintf("GRANT %s ON %s TO %s;", strings.Join(right, ", "), on, user)
		grants = append(grants, grant)
	}

	return fmt.Sprintf("\n"+
		"DROP USER IF EXISTS %s;\n"+
		"CREATE USER %s;\n"+
		"ALTER USER %s IDENTIFIED BY '%s';\n"+
		"%s", // GRANTs statements
		user, user, user, pass, strings.Join(grants, "\n"))
}
