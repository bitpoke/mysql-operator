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

// RunTakeBackupCommand starts a backup command
func RunTakeBackupCommand(cfg *Config, srcHost, destBucket string) error {
	log.Info("take a backup", "host", srcHost, "bucket", destBucket)
	destBucket = normalizeBucketURI(destBucket)
	return pushBackupFromTo(cfg, srcHost, destBucket)
}

func pushBackupFromTo(cfg *Config, srcHost, destBucket string) error {
	tmpDestBucket := fmt.Sprintf("%s.tmp", destBucket)

	response, err := requestABackup(cfg, srcHost, serverBackupEndpoint)
	if err != nil {
		return fmt.Errorf("getting backup: %s", err)
	}

	compressCmd := cfg.BackupCompressCmd()
	// nolint: gosec
	compress := exec.Command(compressCmd[0], compressCmd[1:]...)

	// nolint: gosec
	rclone := exec.Command("rclone", append(cfg.RcloneArgs(), "rcat", tmpDestBucket)...)

	compress.Stdin = response.Body
	compress.Stderr = os.Stderr
	rclone.Stderr = os.Stderr

	if rclone.Stdin, err = compress.StdoutPipe(); err != nil {
		return err
	}

	errChan := make(chan error, 2)

	go func() {
		log.V(2).Info("wait for compress to finish")
		errChan <- compress.Run()
	}()

	go func() {
		log.V(2).Info("wait for rclone to finish")
		errChan <- rclone.Run()
	}()

	// wait for both commands to finish successful
	for i := 1; i <= 2; i++ {
		if err = <-errChan; err != nil {
			return err
		}
	}

	if err = checkBackupTrailers(response); err != nil {
		// backup failed so delete it from remote
		log.Info("backup was partially taken", "trailers", response.Trailer)
		return err
	}

	log.Info("backup was taken successfully, now move it to permanent URL")

	// the backup was a success
	// remove .tmp extension
	// nolint: gosec
	rclone = exec.Command("rclone", append(cfg.RcloneArgs(), "moveto", tmpDestBucket, destBucket)...)

	if err = rclone.Start(); err != nil {
		return fmt.Errorf("final move failed: %s", err)
	}

	if err = rclone.Wait(); err != nil {
		return fmt.Errorf("final move failed: %s", err)
	}

	return nil
}

func normalizeBucketURI(bucket string) string {
	return strings.Replace(bucket, "://", ":", 1)
}
