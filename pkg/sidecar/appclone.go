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
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

// RunCloneCommand clones the data from several potential sources.
//
// There are a few possible scenarios that this function tries to handle:
//
//  Scenario                 | Action Taken
// ------------------------------------------------------------------------------------
// Data already exists       | Log an informational message and return without error.
//                           | This permits the pod to continue initializing and mysql
//                           | will use the data already on the PVC.
// ------------------------------------------------------------------------------------
// Healthy replicas exist    | We will attempt to clone from the healthy replicas.
//                           | If the cloning starts but is interrupted, we will return
//                           | with an error, not trying to clone from the master. The
//                           | assumption is that some intermittent error caused the
//                           | failure and we should let K8S restart the init container
//                           | to try to clone from the replicas again.
// ------------------------------------------------------------------------------------
// No healthy replicas; only | We attempt to clone from the master, assuming that this
// master exists             | is the initialization of the second pod in a multi-pod
//                           | cluster. If cloning starts and is interrupted, we will
//                           | return with an error, letting K8S try again.
// ------------------------------------------------------------------------------------
// No healthy replicas; no   | If there is a bucket URL to clone from, we will try that.
// master; bucket URL exists | The assumption is that this is the bootstrap case: the
//                           | very first mysql pod is being initialized.
// ------------------------------------------------------------------------------------
// No healthy replicas; no   | If this is the first pod in the cluster, then allow it
// master; no bucket URL     | to initialize as an empty instance, otherwise, return an
//                           | error to allow k8s to kill and restart the pod.
// ------------------------------------------------------------------------------------
func RunCloneCommand(cfg *Config) error {
	log.Info("cloning command", "host", cfg.Hostname)

	if cfg.ExistsMySQLData {
		log.Info("data already exists! Remove manually PVC to cleanup or to reinitialize.")
		return nil
	}

	if err := deleteLostFound(); err != nil {
		return fmt.Errorf("removing lost+found: %s", err)
	}

	if isServiceAvailable(cfg.ReplicasFQDN()) {
		if err := attemptClone(cfg, cfg.ReplicasFQDN()); err != nil {
			return fmt.Errorf("cloning from healthy replicas failed due to unexpected error: %s", err)
		}
	} else if isServiceAvailable(cfg.MasterFQDN()) {
		log.Info("healthy replica service was unavailable for cloning, will attempt to clone from the master")
		if err := attemptClone(cfg, cfg.MasterFQDN()); err != nil {
			return fmt.Errorf("cloning from master service failed due to unexpected error: %s", err)
		}
	} else if cfg.ShouldCloneFromBucket() {
		// cloning from provided initBucketURL
		log.Info("cloning from bucket")
		if err := cloneFromBucket(cfg); err != nil {
			return fmt.Errorf("failed to clone from bucket, err: %s", err)
		}
	} else if cfg.IsFirstPodInSet() {
		log.Info("nothing to clone from: empty cluster initializing")
		return nil
	} else {
		return fmt.Errorf("nothing to clone from: no existing data found, no replicas and no master available, and no clone bucket url found")
	}

	// prepare backup
	return xtrabackupPrepare(cfg)
}

func isServiceAvailable(svc string) bool {
	req, err := http.NewRequest("GET", prepareURL(svc, serverProbeEndpoint), nil)
	if err != nil {
		log.Info("failed to check available service", "service", svc, "error", err)
		return false
	}

	client := &http.Client{}
	client.Transport = transportWithTimeout(serverConnectTimeout)
	resp, err := client.Do(req)
	if err != nil {
		log.Info("service was not available", "service", svc, "error", err)
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		log.Info("service not available", "service", svc, "HTTP status code", resp.StatusCode)
		return false
	}

	return true
}

func attemptClone(cfg *Config, sourceService string) error {
	err := cloneFromSource(cfg, sourceService)
	if err != nil {
		return fmt.Errorf("failed to clone from %s, err: %s", sourceService, err)
	}
	return nil
}

func cloneFromBucket(cfg *Config) error {
	initBucket := strings.Replace(cfg.InitBucketURL, "://", ":", 1)

	log.Info("cloning from bucket", "bucket", initBucket)

	if _, err := os.Stat(constants.RcloneConfigFile); os.IsNotExist(err) {
		log.Error(err, "rclone config file does not exists")
		return err
	}

	// writes the contents of the bucket url to stdout
	// nolint: gosec
	rclone := exec.Command("rclone", append(cfg.RcloneArgs(), "cat", initBucket)...)

	// gzip reads from stdin decompress and then writes to stdout
	// nolint: gosec
	gzip := exec.Command("gzip", "-d")

	// extracts files from stdin and writes them to mysql data dir
	// nolint: gosec
	xbstream := exec.Command(xbstreamCommand, cfg.XbstreamArgs()...)

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

	// extracts files from stdin and writes them to mysql data dir
	// nolint: gosec
	xbstream := exec.Command(xbstreamCommand, cfg.XbstreamArgs()...)

	xbstream.Stdin = response.Body
	xbstream.Stderr = os.Stderr

	cloneSucceeded := false
	defer func() {
		if !cloneSucceeded {
			log.Info("clone operation failed, cleaning up dataDir so retries may proceed")
			cleanDataDir()
		}
	}()

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
	}

	if err := checkBackupTrailers(response); err != nil {
		return err
	}

	cloneSucceeded = true
	return nil
}

func xtrabackupPrepare(cfg *Config) error {
	// nolint: gosec
	xtrabackupPrepare := exec.Command(xtrabackupCommand, cfg.XtrabackupPrepareArgs()...)
	xtrabackupPrepare.Stderr = os.Stderr

	return xtrabackupPrepare.Run()
}

func deleteLostFound() error {
	lfPath := fmt.Sprintf("%s/lost+found", dataDir)
	return os.RemoveAll(lfPath)
}

func cleanDataDir() {
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		log.Error(err, "failed to clean dataDir")
	}

	for _, f := range files {
		toRemove := path.Join(dataDir, f.Name())
		if err := os.RemoveAll(toRemove); err != nil {
			log.Error(err, "failed to remove file in dataDir")
		}
	}
}
