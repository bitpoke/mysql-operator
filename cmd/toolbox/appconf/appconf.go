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
	"strconv"

	"github.com/go-ini/ini"
	"github.com/golang/glog"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
	"github.com/presslabs/titanium/pkg/util"
)

const (
	MountConf = "/mnt/conf"
	ConfDir   = "/etc/mysql"

	rStrLen = 18
)

// RunInitCommand generates my.cnf file.
// With server-id, utility-user, and utility-user-password.
func RunConfigCommand(stopCh <-chan struct{}) error {
	role := tb.NodeRole()
	glog.Infof("Configuring server: %s as %s", tb.GetHostname(), role)

	cfg, err := ini.Load(MountConf + "/server-cnf")
	if err != nil {
		return fmt.Errorf("failed to load configs, err: %s", err)
	}

	mysqld, err := cfg.GetSection("mysqld")
	if err != nil {
		return fmt.Errorf("failed to load configs, err: %s", err)
	}

	uName := util.RandomString(rStrLen)
	uPass := util.RandomString(rStrLen)

	mysqld.NewKey("server-id", strconv.Itoa(tb.GetServerId()))
	mysqld.NewKey("utility-user", uName)
	mysqld.NewKey("utility-user-password", uPass)

	client, err := cfg.GetSection("client")
	if err != nil {
		return fmt.Errorf("failed to load configs, err: %s", err)
	}
	client.NewKey("user", uName)
	client.NewKey("password", uPass)

	err = cfg.SaveTo(ConfDir + "/my.cnf")
	if err != nil {
		return fmt.Errorf("failed to save configs, err: %s", err)
	}
	return nil
}
