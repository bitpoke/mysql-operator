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

package app

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("sidecar.app")

// RunQuery executes a query
func RunQuery(cfg *MysqlConfig, q string, args ...interface{}) error {
	if cfg.MysqlDSN == nil {
		log.Info("could not get mysql connection DSN")
		return fmt.Errorf("no DSN specified")
	}

	db, err := sql.Open("mysql", *cfg.MysqlDSN)
	if err != nil {
		return err
	}

	log.V(4).Info("running query", "query", q, "args", args)
	if _, err := db.Exec(q, args...); err != nil {
		return err
	}

	return nil
}

// CopyFile the src file to dst. Any existing file will be overwritten and will not
// copy file attributes.
// nolint: gosec
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err1 := in.Close(); err1 != nil {
			log.Error(err1, "failed to close source file", "src_file", src)
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err1 := out.Close(); err1 != nil {
			log.Error(err1, "failed to close destination file", "dest_file", dst)
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}

// MaxClients limit an http endpoint to allow just n max concurrent connections
func MaxClients(h http.Handler, n int) http.Handler {
	sema := make(chan struct{}, n)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sema <- struct{}{}
		defer func() { <-sema }()

		h.ServeHTTP(w, r)
	})
}

// RequestABackup connects to specified host and endpoint and gets the backup
func RequestABackup(cfg *BaseConfig, host, endpoint string) (io.Reader, error) {
	log.Info("initialize a backup", "host", host, "endpoint", endpoint)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d%s", host, ServerPort, endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("fail to create request: %s", err)
	}

	// set authentification user and password
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

	return resp.Body, nil
}

// ReadPurgedGTID returns the GTID from xtrabackup_binlog_info file
func ReadPurgedGTID() (string, error) {
	file, err := os.Open(fmt.Sprintf("%s/xtrabackup_binlog_info", DataDir))
	if err != nil {
		return "", err
	}

	defer func() {
		if err1 := file.Close(); err1 != nil {
			log.Error(err1, "failed to close file")
		}
	}()

	return getGTIDFrom(file)
}

// getGTIDFrom parse the content from xtrabackup_binlog_info file passed as
// io.Reader and extracts the GTID.
func getGTIDFrom(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanWords)

	count := 0
	gtid := ""
	for scanner.Scan() {
		if count == 2 {
			gtid = scanner.Text()
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	} else if len(gtid) == 0 {
		return "", fmt.Errorf("failed to read GTID reached EOF")
	}

	return gtid, nil
}

// ShouldBootstrapNode checks if the mysql data is at the first initialization
func ShouldBootstrapNode() bool {
	_, err := os.Open(fmt.Sprintf("%s/%s/%s.CSV", DataDir,
		ToolsDbName, ToolsInitTableName))
	if os.IsNotExist(err) {
		return true
	} else if err != nil {
		log.Error(err, "first init check failed hard")
		return true
	}

	// maybe check csv init data and log it
	return false
}
