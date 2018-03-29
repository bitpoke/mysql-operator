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

// RunInitCommand generates my.cnf file.
// With server-id, utility-user, and utility-user-password.
func RunConfigCommand(stopCh <-chan struct{}) error {
	role := tb.NodeRole()
	glog.Infof("Configuring server: %s as %s", tb.GetHostname(), role)

	if err := tb.CopyFile(tb.MountConfigDir+"/my.cnf", tb.ConfigDir+"/my.cnf"); err != nil {
		return fmt.Errorf("copy file my.cnf: %s", err)
	}

	uPass := util.RandomString(rStrLen)
	reportHost := tb.GetHostFor(tb.GetServerId())
	dynCFG := getDynamicConfigs(tb.GetServerId(), reportHost)

	if err := os.Mkdir(tb.ConfDPath, os.FileMode(0755)); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error mkdir %s/conf.d: %s", tb.ConfigDir, err)
		}
	}
	if err := dynCFG.SaveTo(tb.ConfDPath + "/10-dynamic.cnf"); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	utilityCFG := getUtilityUserConfigs(tb.UtilityUser, uPass)
	if err := utilityCFG.SaveTo(tb.ConfDPath + "/10-utility-user.cnf"); err != nil {
		return fmt.Errorf("failed to configure utility user: %s", err)
	}

	clientCFG := getClientConfigs(tb.UtilityUser, uPass)
	if err := clientCFG.SaveTo(tb.ConfigDir + "/client.cnf"); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	return nil
}

func getClientConfigs(user, pass string) *ini.File {
	cfg := ini.Empty()
	// create file /etc/mysql/client.cnf
	client := cfg.Section("client")

	client.NewKey("host", "127.0.0.1")
	client.NewKey("port", tb.MysqlPort)
	client.NewKey("user", user)
	client.NewKey("password", pass)

	return cfg
}

func getDynamicConfigs(id int, reportHost string) *ini.File {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	mysqld.NewKey("server-id", strconv.Itoa(id))
	mysqld.NewKey("report-host", reportHost)

	return cfg
}

func getUtilityUserConfigs(user, pass string) *ini.File {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	mysqld.NewKey("utility-user", fmt.Sprintf("%s@%%", user))
	mysqld.NewKey("utility-user-password", pass)
	mysqld.NewKey("utility-user-schema-access", "mysql")
	mysqld.NewKey("utility-user-privileges",
		"SELECT,INSERT,UPDATE,DELETE,CREATE,DROP,GRANT,ALTER,SHOW DATABASES,SUPER,CREATE USER,"+
			"PROCESS,RELOAD,LOCK TABLES,REPLICATION CLIENT,REPLICATION SLAVE",
	)

	return cfg
}
