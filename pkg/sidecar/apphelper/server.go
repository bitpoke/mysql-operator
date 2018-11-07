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

package apphelper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/presslabs/mysql-operator/pkg/sidecar/util"
)

type server struct {
	http.Server
}

func newServer(stop <-chan struct{}) *server {
	mux := http.NewServeMux()
	srv := &server{
		Server: http.Server{
			Addr:    fmt.Sprintf(":%d", util.ServerPort),
			Handler: mux,
		},
	}

	// Add handle functions
	mux.HandleFunc(util.ServerProbeEndpoint, srv.healthHandler)
	mux.Handle(util.ServerBackupEndpoint, util.MaxClients(http.HandlerFunc(srv.backupHandler), 1))

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
		http.Error(w, "Streamming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Connection", "keep-alive")

	// nolint: gosec
	xtrabackup := exec.Command("xtrabackup", "--backup", "--slave-info", "--stream=xbstream",
		"--host=127.0.0.1", fmt.Sprintf("--user=%s", util.GetReplUser()),
		fmt.Sprintf("--password=%s", util.GetReplPass()))

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
		log.Error(err, "failed to start extrabackup command")
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(w, stdout); err != nil {
		log.Error(err, "failed to copy buffer")
		http.Error(w, "buffer copy failed", http.StatusInternalServerError)
		return
	}

	flusher.Flush()

	if err := xtrabackup.Wait(); err != nil {
		log.Error(err, "failed waiting for xtrabackup to finish")
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}
}

func (s *server) isAuthenticated(r *http.Request) bool {
	user, pass, ok := r.BasicAuth()
	return ok && user == util.GetBackupUser() && pass == util.GetBackupPass()
}
