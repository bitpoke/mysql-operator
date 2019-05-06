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
	"os"
	"os/exec"
	"strings"
)

// RunCloneCommand clone the data from source.
// nolint: gocyclo
func RunCloneCommand(cfg *Config) error {
	log.Info("cloning command", "host", cfg.Hostname)

	// skip cloning if data exists.
	if !shouldBootstrapNode() {
		log.Info("data exists and is initialized, skipping cloning.")
		return nil
	}

	if checkIfDataExists() {
		log.Info("data already exists! Remove manually PVC to cleanup or to reinitialize.")
		return nil
	}

	if err := deleteLostFound(); err != nil {
		return fmt.Errorf("removing lost+found: %s", err)
	}

	if cfg.NodeRole() == MasterNode {
		if len(cfg.InitBucketURL) == 0 {
			log.Info("skip cloning init bucket uri is not set.")
			// let mysqld initialize data dir
			return nil
		}
		err := cloneFromBucket(cfg.InitBucketURL)
		if err != nil {
			return fmt.Errorf("failed to clone from bucket, err: %s", err)
		}
	} else {
		// clonging from prior node
		if cfg.ServerID() > 100 {
			sourceHost := cfg.FQDNForServer(cfg.ServerID() - 1)
			err := cloneFromSource(cfg, sourceHost)
			if err != nil {
				return fmt.Errorf("failed to clone from %s, err: %s", sourceHost, err)
			}
			// clone from master if no prior node and node is not master
		} else if cfg.ServerID() == 100 {
			sourceHost := cfg.MasterFQDN()
			err := cloneFromSource(cfg, sourceHost)
			if err != nil {
				return fmt.Errorf("failed to clone from %s, err: %s", sourceHost, err)
			}
		} else {
			return fmt.Errorf(
				"failed to initialize because no prior node exists, check orchestrator maybe",
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

	log.Info("cloning from bucket", "bucket", initBucket)

	if _, err := os.Stat(rcloneConfigFile); os.IsNotExist(err) {
		log.Error(err, "rclone config file does not exists")
		return err
	}
	// rclone --config={conf file} cat {bucket uri}
	// writes to stdout the content of the bucket uri
	// nolint: gosec
	rclone := exec.Command("rclone", "-vv",
		fmt.Sprintf("--config=%s", rcloneConfigFile), "cat", initBucket)

	// gzip reads from stdin decompress and then writes to stdout
	// nolint: gosec
	gzip := exec.Command("gzip", "-d")

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	// nolint: gosec
	xbstream := exec.Command("xbstream", "-x", "-C", dataDir)

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

	log.Info("cloning done successfully")
	return nil
}

func cloneFromSource(cfg *Config, host string) error {
	log.Info("cloning from node", "host", host)

	response, err := requestABackup(cfg, host, serverBackupEndpoint)
	if err != nil {
		return fmt.Errorf("fail to get backup: %s", err)
	}

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	// nolint: gosec
	xbstream := exec.Command("xbstream", "-x", "-C", dataDir)

	xbstream.Stdin = response.Body
	xbstream.Stderr = os.Stderr

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
	}

	if err := checkBackupTrailers(response); err != nil {
		return err
	}

	return nil
}

func xtrabackupPreperData() error {
	// nolint: gosec
	xtbkCmd := exec.Command("xtrabackup", "--prepare",
		fmt.Sprintf("--target-dir=%s", dataDir))

	xtbkCmd.Stderr = os.Stderr

	return xtbkCmd.Run()
}

// nolint: gosec
func checkIfDataExists() bool {
	path := fmt.Sprintf("%s/mysql", dataDir)
	_, err := os.Open(path)

	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.Error(err, "failed to open file", "file", path)
	}

	return true
}

func deleteLostFound() error {
	path := fmt.Sprintf("%s/lost+found", dataDir)
	return os.RemoveAll(path)
}
