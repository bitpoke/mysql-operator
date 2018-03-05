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

package appservebackup

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/radovskyb/watcher"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

// RunServerBackupCommand watch for orchestrator topology creds for change and
// serve backups.
func RunServeBackupCommand(stopCh <-chan struct{}) error {

	if tb.NodeRole() == "master" && len(tb.GetOrcUser()) > 0 {
		glog.Info("Watch for orc creds if changes.")
		w, err := syncOrcUserPass(stopCh)
		if err != nil {
			return fmt.Errorf(
				"fail to set watcher for orchestrator topology changes, err: %s", err)
		}
		defer w.Close()
	}

	glog.Infof("Serve backups command.")

	xtrabackup_cmd := []string{"xtrabackup", "--backup", "--slave-info", "--stream=xbstream",
		"--host=127.0.0.1", fmt.Sprintf("--user=%s", tb.GetReplUser()),
		fmt.Sprintf("--password=%s", tb.GetReplPass())}

	ncat := exec.Command("ncat", "--listen", "--keep-open", "--send-only", "--max-conns=1",
		tb.BackupPort, "-c", strings.Join(xtrabackup_cmd, " "))

	return ncat.Run()
}

func syncOrcUserPass(stopCh <-chan struct{}) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)

	go func() {
		for {
			select {
			case <-w.Event:
				glog.Info("topology creds changed, syncing..")
				if err := tb.UpdateOrcUserPass(); err != nil {
					glog.Errorf("Failed to update orc user and pass with err: %s", err)
				}
			case err := <-w.Error:
				glog.V(2).Info("Got error when watching orc topology creds, err: %s", err)

			case <-stopCh:
				return

			case <-w.Closed:
				return
			}
		}
	}()

	if err := w.Add(tb.OrcTopologyDir + "/TOPOLOGY_PASSWORD"); err != nil {
		return nil, err
	}

	if err := w.Add(tb.OrcTopologyDir + "/TOPOLOGY_USER"); err != nil {
		return nil, err
	}

	go func() {
		if err := w.Start(time.Second * 5); err != nil {
			glog.Errorf("faild to start watcher, err: %s", err)
		}
	}()

	return w, nil

}
