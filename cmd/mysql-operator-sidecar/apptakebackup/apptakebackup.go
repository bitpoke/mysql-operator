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

package apptakebackup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"

	"github.com/presslabs/mysql-operator/cmd/mysql-operator-sidecar/util"
)

// RunTakeBackupCommand starts a backup command
// nolint: unparam
func RunTakeBackupCommand(stopCh <-chan struct{}, srcHost, destBucket string) error {
	glog.Infof("Taking backup from '%s' to bucket '%s' ...", srcHost, destBucket)
	destBucket = normalizeBucketURI(destBucket)
	return pushBackupFromTo(srcHost, destBucket)
}

// nolint: gas
func pushBackupFromTo(srcHost, destBucket string) error {

	backupBody, err := util.RequestABackup(srcHost, util.ServerBackupEndpoint)
	if err != nil {
		return fmt.Errorf("getting backup: %s", err)
	}

	gzip := exec.Command("gzip", "-c")

	rclone := exec.Command("rclone",
		fmt.Sprintf("--config=%s", util.RcloneConfigFile), "rcat", destBucket)

	gzip.Stdin = backupBody
	gzip.Stderr = os.Stderr
	rclone.Stderr = os.Stderr

	if rclone.Stdin, err = gzip.StdoutPipe(); err != nil {
		return err
	}

	if err := gzip.Start(); err != nil {
		return fmt.Errorf("gzip start error: %s", err)
	}

	if err := rclone.Start(); err != nil {
		return fmt.Errorf("rclone start error: %s", err)
	}

	glog.V(2).Info("Wait for gzip to finish.")
	if err := gzip.Wait(); err != nil {
		return fmt.Errorf("gzip wait error: %s", err)
	}

	glog.V(2).Info("Wait for rclone to finish.")
	if err := rclone.Wait(); err != nil {
		return fmt.Errorf("rclone wait error: %s", err)
	}

	return nil
}

func normalizeBucketURI(bucket string) string {
	return strings.Replace(bucket, "://", ":", 1)
}
