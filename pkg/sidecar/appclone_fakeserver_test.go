/*
Copyright 2019 Harvest

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
	"net/http"
	"time"
)

type loggedRequest struct {
	endpoint  string
	timestamp time.Time
}

type fakeServer struct {
	cfg              *Config
	server           http.Server
	calls            []loggedRequest
	simulateTruncate bool // Will cause the next request to truncate the response
	simulateError    bool // Will cause the next request to return http error
	validXbstream    []byte
}

func newFakeServer(address string, cfg *Config) *fakeServer {
	mux := http.NewServeMux()
	fSrv := &fakeServer{
		cfg: cfg,
		server: http.Server{
			Addr:    address,
			Handler: mux,
		},
	}

	// A small file named "t" containing the text "fake-backup", encoded with `xbstream --create`
	fSrv.validXbstream = []byte{
		0x58, 0x42, 0x53, 0x54, 0x43, 0x4b, 0x30, 0x31, 0x00, 0x50, 0x01, 0x00, 0x00, 0x00, 0x74, 0x0c, // XBSTCK01.P....t.
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x6b, // ...............k
		0xcc, 0x84, 0x00, 0x66, 0x61, 0x6b, 0x65, 0x2d, 0x62, 0x61, 0x63, 0x6b, 0x75, 0x70, 0x0a, 0x58, // ...fake-backup.X
		0x42, 0x53, 0x54, 0x43, 0x4b, 0x30, 0x31, 0x00, 0x45, 0x01, 0x00, 0x00, 0x00, 0x74, // BSTCK01.E....t.
	}

	fSrv.reset()

	mux.Handle(serverProbeEndpoint, http.HandlerFunc(fSrv.healthHandler))
	mux.Handle(serverBackupEndpoint, http.HandlerFunc(fSrv.backupHandler))

	return fSrv
}

// Since we are starting/stopping these fake servers for individual test cases, we should wait
// for them to startup so as to avoid false positives in our tests.
func (fSrv *fakeServer) waitReady() error {
	retries := 0
	for {
		resp, err := http.Get(prepareURL(fSrv.server.Addr, serverProbeEndpoint))
		if err == nil && resp.StatusCode == 200 {
			return nil
		}
		if retries++; retries > 5 {
			return fmt.Errorf("could not start fake sidecar server: %s", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (fSrv *fakeServer) start() error {
	go func() {
		err := fSrv.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic("couldn't start fakeserver")
		}
	}()
	return fSrv.waitReady()
}

func (fSrv *fakeServer) stop() error {
	if err := fSrv.server.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("failed to stop appclone test server: %s", err)
	}
	return nil
}

func (fSrv *fakeServer) reset() {
	fSrv.calls = make([]loggedRequest, 0)
	fSrv.simulateError = false
	fSrv.simulateTruncate = false
}

func (fSrv *fakeServer) backupRequestsReceived() int {
	return fSrv.callsForEndpoint(serverBackupEndpoint)
}

func (fSrv *fakeServer) callsForEndpoint(endpoint string) int {
	count := 0
	for _, call := range fSrv.calls {
		if call.endpoint == endpoint {
			count++
		}
	}
	return count
}

func (fSrv *fakeServer) healthHandler(w http.ResponseWriter, req *http.Request) {
	fSrv.calls = append(fSrv.calls, loggedRequest{req.RequestURI, time.Now()})
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Error(err, "failed writing request")
	}
}

func (fSrv *fakeServer) backupHandler(w http.ResponseWriter, req *http.Request) {
	fSrv.calls = append(fSrv.calls, loggedRequest{req.RequestURI, time.Now()})

	// Error: return http status code of 500
	if fSrv.simulateError {
		http.Error(w, "xtrabackup failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Trailer", backupStatusTrailer)

	backup := fSrv.validXbstream
	// Truncate: send half the stream, with "successful" trailers
	if fSrv.simulateTruncate {
		backup = fSrv.validXbstream[0:10]
	}

	if _, err := w.Write(backup); err != nil {
		log.Error(err, "failed writing request")
	}

	w.Header().Set(backupStatusTrailer, backupSuccessful)
}
