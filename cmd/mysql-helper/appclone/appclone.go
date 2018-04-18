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

package appclone

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"

	tb "github.com/presslabs/mysql-operator/cmd/mysql-helper/util"
)

// RunCloneCommand clone the data from source.
func RunCloneCommand(stopCh <-chan struct{}) error {
	glog.Infof("Cloning into node %s", tb.GetHostname())

	// skip cloning if data exists.
	init, err := checkIfWasInit()
	if err != nil {
		return fmt.Errorf("failed to read init file: %s", err)
	} else if init {
		glog.Info("Data exists and is initialized. Skip clonging.")
		return nil
	} else {
		if checkIfDataExists() {
			glog.Infof("Data alerady exists! Remove manualy PVC to cleanup or to reinitialize.")
			return nil
		}
	}

	if err := deleteLostFound(); err != nil {
		return fmt.Errorf("removing lost+found: %s", err)
	}

	if tb.NodeRole() == "master" {
		initBucket := tb.GetInitBucket()
		if len(initBucket) == 0 {
			glog.Info("Skip cloning init bucket uri is not set.")
			// let mysqld initialize data dir
			return nil
		}
		err := cloneFromBucket(initBucket)
		if err != nil {
			return fmt.Errorf("failed to clone from bucket, err: %s", err)
		}
	} else {
		// clonging from prior node
		if tb.GetServerId() > 100 {
			sourceHost := tb.GetHostFor(tb.GetServerId() - 1)
			err := cloneFromSource(sourceHost)
			if err != nil {
				return fmt.Errorf("failed to clone from %s, err: %s", sourceHost, err)
			}
		} else {
			return fmt.Errorf(
				"failed to initialize because no of no prior node exists, check orchestrator maybe",
			)
		}
	}

	// prepare backup
	if err := xtrabackupPreperData(); err != nil {
		return err
	}

	return nil
}

func cloneFromBucket(initBucket string) error {
	initBucket = strings.Replace(initBucket, "://", ":", 1)

	glog.Infof("Cloning from bucket: %s", initBucket)

	if _, err := os.Stat(tb.RcloneConfigFile); os.IsNotExist(err) {
		glog.Fatalf("Rclone config file does not exists. err: %s", err)
		return err
	}
	// rclone --config={conf file} cat {bucket uri}
	// writes to stdout the content of the bucket uri
	rclone := exec.Command("rclone", "-vv",
		fmt.Sprintf("--config=%s", tb.RcloneConfigFile), "cat", initBucket)

	// gzip reads from stdin decompress and then writes to stdout
	gzip := exec.Command("gzip", "-d")

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", tb.DataDir)

	var err error
	// rclone | gzip | xbstream
	if gzip.Stdin, err = rclone.StdoutPipe(); err != nil {
		return err
	}

	if xbstream.Stdin, err = gzip.StdoutPipe(); err != nil {
		return err
	}

	rclone.Stderr = os.Stderr
	gzip.Stderr = os.Stderr
	xbstream.Stderr = os.Stderr

	if err := rclone.Start(); err != nil {
		return fmt.Errorf("rclone start error: %s", err)
	}

	if err := gzip.Start(); err != nil {
		return fmt.Errorf("gzip start error: %s", err)
	}

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := rclone.Wait(); err != nil {
		return fmt.Errorf("rclone wait error: %s", err)
	}

	if err := gzip.Wait(); err != nil {
		return fmt.Errorf("gzip wait error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
	}

	glog.Info("Cloning done successfully.")
	return nil
}

func cloneFromSource(host string) error {
	glog.Infof("Cloning from node: %s", host)
	// ncat --recv-only {host} {port}
	// connects to host and get data from there
	ncat := exec.Command("ncat", "--recv-only", host, tb.BackupPort)

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", tb.DataDir)

	ncat.Stderr = os.Stderr
	xbstream.Stderr = os.Stderr

	var err error
	if xbstream.Stdin, err = ncat.StdoutPipe(); err != nil {
		return fmt.Errorf("set pipe, error: %s", err)
	}

	if err := ncat.Start(); err != nil {
		return fmt.Errorf("ncat start error: %s", err)
	}

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := ncat.Wait(); err != nil {
		return fmt.Errorf("ncat wait error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
	}

	return nil
}

func xtrabackupPreperData() error {
	replUser := tb.GetReplUser()
	replPass := tb.GetReplPass()

	// TODO: remove user and password for here, not needed.
	xtbkCmd := exec.Command("xtrabackup", "--prepare",
		fmt.Sprintf("--target-dir=%s", tb.DataDir),
		fmt.Sprintf("--user=%s", replUser), fmt.Sprintf("--password=%s", replPass))

	xtbkCmd.Stderr = os.Stderr

	return xtbkCmd.Run()
}

func checkIfWasInit() (bool, error) {
	_, err := os.Open(fmt.Sprintf("%s/%s/%s.CSV", tb.DataDir, tb.ToolsDbName,
		tb.ToolsInitTableName))

	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// maybe check csv init data and log it

	return true, nil
}

func checkIfDataExists() bool {
	path := fmt.Sprintf("%s/mysql", tb.DataDir)
	_, err := os.Open(path)

	if os.IsNotExist(err) {
		return false
	} else {
		glog.Warning("[checkIfDataExists] faild to open %s with err: %s", path, err)
	}

	return true
}

func deleteLostFound() error {
	path := fmt.Sprintf("%s/lost+found", tb.DataDir)
	return os.RemoveAll(path)
}
