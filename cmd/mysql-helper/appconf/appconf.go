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

package appconf

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-ini/ini"
	"github.com/golang/glog"

	tb "github.com/presslabs/mysql-operator/cmd/mysql-helper/util"
	"github.com/presslabs/mysql-operator/pkg/util"
)

const (
	rStrLen = 18
)

// RunConfigCommand generates my.cnf, client.cnf and 10-dynamic.cnf files.
func RunConfigCommand(stopCh <-chan struct{}) error {
	role := tb.NodeRole()
	glog.Infof("Configuring server: %s as %s", tb.GetHostname(), role)

	if err := tb.CopyFile(tb.MountConfigDir+"/my.cnf", tb.ConfigDir+"/my.cnf"); err != nil {
		return fmt.Errorf("copy file my.cnf: %s", err)
	}

	uPass := util.RandomString(rStrLen)
	reportHost := tb.GetHostFor(tb.GetServerID())

	var dynCFG, utilityCFG, clientCFG *ini.File
	var err error

	if dynCFG, err = getDynamicConfigs(tb.GetServerID(), reportHost); err != nil {
		return fmt.Errorf("failed to get dynamic configs: %s", err)
	}

	if err = os.Mkdir(tb.ConfDPath, os.FileMode(0755)); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error mkdir %s/conf.d: %s", tb.ConfigDir, err)
		}
	}
	if err = dynCFG.SaveTo(tb.ConfDPath + "/10-dynamic.cnf"); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}
	if utilityCFG, err = getUtilityUserConfigs(tb.UtilityUser, uPass); err != nil {
		return fmt.Errorf("failed to configure utility user: %s", err)
	}
	if err = utilityCFG.SaveTo(tb.ConfDPath + "/10-utility-user.cnf"); err != nil {
		return fmt.Errorf("failed to configure utility user: %s", err)
	}

	if clientCFG, err = getClientConfigs(tb.UtilityUser, uPass); err != nil {
		return fmt.Errorf("failed to get client configs: %s", err)
	}

	if err = clientCFG.SaveTo(tb.ConfigDir + "/client.cnf"); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	return nil
}

func getClientConfigs(user, pass string) (*ini.File, error) {
	cfg := ini.Empty()
	// create file /etc/mysql/client.cnf
	client := cfg.Section("client")

	if _, err := client.NewKey("host", "127.0.0.1"); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("port", tb.MysqlPort); err != nil {
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

func getDynamicConfigs(id int, reportHost string) (*ini.File, error) {
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

func getUtilityUserConfigs(user, pass string) (*ini.File, error) {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	if _, err := mysqld.NewKey("utility-user", fmt.Sprintf("%s@%%", user)); err != nil {
		return nil, err
	}
	if _, err := mysqld.NewKey("utility-user-password", pass); err != nil {
		return nil, err
	}
	if _, err := mysqld.NewKey("utility-user-schema-access", "mysql"); err != nil {
		return nil, err
	}
	if _, err := mysqld.NewKey("utility-user-privileges",
		"SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,GRANT,ALTER,SHOW DATABASES,SUPER,CREATE USER,"+
			"PROCESS,RELOAD,LOCK TABLES,REPLICATION CLIENT,REPLICATION SLAVE",
	); err != nil {
		return nil, err
	}

	return cfg, nil
}
