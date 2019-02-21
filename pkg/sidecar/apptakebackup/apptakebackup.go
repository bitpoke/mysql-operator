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

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/mysql-operator/pkg/sidecar/app"
)

var log = logf.Log.WithName("sidecar.apptakebackup")

// RunTakeBackupCommand starts a backup command
// nolint: unparam
func RunTakeBackupCommand(cfg *app.BaseConfig, srcHost, destBucket string) error {
	log.Info("take a backup", "host", srcHost, "bucket", destBucket)
	destBucket = normalizeBucketURI(destBucket)
	return pushBackupFromTo(cfg, srcHost, destBucket)
}

func pushBackupFromTo(cfg *app.BaseConfig, srcHost, destBucket string) error {

	backupBody, err := app.RequestABackup(cfg, srcHost, app.ServerBackupEndpoint)
	if err != nil {
		return fmt.Errorf("getting backup: %s", err)
	}

	// nolint: gosec
	gzip := exec.Command("gzip", "-c")

	// nolint: gosec
	rclone := exec.Command("rclone",
		fmt.Sprintf("--config=%s", app.RcloneConfigFile), "rcat", destBucket)

	gzip.Stdin = backupBody
	gzip.Stderr = os.Stderr
	rclone.Stderr = os.Stderr

	if rclone.Stdin, err = gzip.StdoutPipe(); err != nil {
		return err
	}

	errChan := make(chan error, 2)

	go func() {
		log.V(2).Info("wait for gzip to finish")
		errChan <- gzip.Run()
	}()

	go func() {
		log.V(2).Info("wait for rclone to finish")
		errChan <- rclone.Run()
	}()

	// wait for both commands to finish successful
	for i := 1; i <= 2; i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func normalizeBucketURI(bucket string) string {
	return strings.Replace(bucket, "://", ":", 1)
}
