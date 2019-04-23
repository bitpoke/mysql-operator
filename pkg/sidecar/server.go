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
	"context"
	"fmt"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
	"io"
	"net/http"
	"os"
	"os/exec"
)

const (
	backupStatusTrailer = "X-Backup-Status"
	backupSuccessful    = "Success"
	backupFailed        = "Failed"
)

type server struct {
	cfg *Config
	http.Server
}

func newServer(cfg *Config, stop <-chan struct{}) *server {
	mux := http.NewServeMux()
	srv := &server{
		cfg: cfg,
		Server: http.Server{
			Addr:    fmt.Sprintf(":%d", serverPort),
			Handler: mux,
		},
	}

	// Add handle functions
	mux.HandleFunc(serverProbeEndpoint, srv.healthHandler)
	mux.Handle(serverBackupEndpoint, maxClients(http.HandlerFunc(srv.backupHandler), 1))

	// Shutdown gracefully the http server
	go func() {
		<-stop // wait for stop signal
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Error(err, "failed to stop http server")

		}
	}()

	return srv
}

// nolint: unparam
func (s *server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Error(err, "failed writing request")
	}
}

func (s *server) backupHandler(w http.ResponseWriter, r *http.Request) {

	if !s.isAuthenticated(r) {
		http.Error(w, "Not authenticated!", http.StatusForbidden)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Trailer", backupStatusTrailer)

	// nolint: gosec
	xtrabackup := exec.Command("xtrabackup", "--backup", "--slave-info", "--stream=xbstream",
		fmt.Sprintf("--tables-exclude=%s.%s", constants.OperatorDbName, constants.OperatorGtidsTableName),
		"--host=127.0.0.1", fmt.Sprintf("--user=%s", s.cfg.ReplicationUser),
		fmt.Sprintf("--password=%s", s.cfg.ReplicationPassword),
		"--target-dir=/tmp/xtrabackup_backupfiles/")

	xtrabackup.Stderr = os.Stderr

	stdout, err := xtrabackup.StdoutPipe()
	if err != nil {
		log.Error(err, "failed to create stdout pipe")
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}

	defer func() {
		// don't care
		_ = stdout.Close()
	}()

	if err := xtrabackup.Start(); err != nil {
		log.Error(err, "failed to start xtrabackup command")
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(w, stdout); err != nil {
		log.Error(err, "failed to copy buffer")
		http.Error(w, "buffer copy failed", http.StatusInternalServerError)
		return
	}

	if err := xtrabackup.Wait(); err != nil {
		log.Error(err, "failed waiting for xtrabackup to finish")
		w.Header().Set(backupStatusTrailer, backupFailed)
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}

	// success
	w.Header().Set(backupStatusTrailer, backupSuccessful)
	flusher.Flush()
}

func (s *server) isAuthenticated(r *http.Request) bool {
	user, pass, ok := r.BasicAuth()
	return ok && user == s.cfg.BackupUser && pass == s.cfg.BackupPassword
}

// maxClients limit an http endpoint to allow just n max concurrent connections
func maxClients(h http.Handler, n int) http.Handler {
	sema := make(chan struct{}, n)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sema <- struct{}{}
		defer func() { <-sema }()

		h.ServeHTTP(w, r)
	})
}

// requestABackup connects to specified host and endpoint and gets the backup
func requestABackup(cfg *Config, host, endpoint string) (*http.Response, error) {
	log.Info("initialize a backup", "host", host, "endpoint", endpoint)

	req, err := http.NewRequest("GET", fmt.Sprintf(
		"http://%s:%d%s", host, serverPort, endpoint), nil)

	if err != nil {
		return nil, fmt.Errorf("fail to create request: %s", err)
	}

	// set authentication user and password
	req.SetBasicAuth(cfg.BackupUser, cfg.BackupPassword)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		status := "unknown"
		if resp != nil {
			status = resp.Status
		}
		return nil, fmt.Errorf("fail to get backup: %s, code: %s", err, status)
	}

	return resp, nil
}

func checkBackupTrailers(resp *http.Response) error {
	if values, ok := resp.Trailer[backupStatusTrailer]; !ok || !stringInSlice(backupSuccessful, values) {
		// backup is failed, remove from remote
		return fmt.Errorf("backup failed to be taken: no 'Success' trailer found")
	}

	return nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
