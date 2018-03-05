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

package appinit

import (
	"fmt"
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

func RunInitCommand(stopCh <-chan struct{}) error {
	glog.Infof("Starting initialization...")

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

	if tb.NodeRole() == "master" {
		// master configs
		query := fmt.Sprintf(
			"GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, "+
				"REPLICATION SLAVE ON *.* TO '%s'@'%%' IDENTIFIED BY '%s'",
			tb.GetReplUser(), tb.GetReplPass())

		if _, err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to configure master node, err: %s", err)
		}

		if len(tb.GetOrcUser()) > 0 {
			if err := tb.UpdateOrcUserPass(); err != nil {
				return err
			}
		}

	} else {
		// slave node
		query := fmt.Sprintf(
			"CHANGE MASTER TO MASTER_AUTO_POSITION=1,"+
				"MASTER_HOST='%s',"+
				"MASTER_USER='%s',"+
				"MASTER_PASSWORD='%s',"+
				"MASTER_CONNECT_RETRY=%d",
			tb.GetMasterHost(), tb.GetReplUser(), tb.GetReplPass(), connRetry)

		if _, err := tb.RunQuery(query); err != nil {
			return fmt.Errorf("failed to configure slave node, err: %s", err)
		}
	}

	return nil
}
