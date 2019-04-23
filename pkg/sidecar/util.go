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
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("sidecar")

// runQuery executes a query
func runQuery(cfg *Config, q string, args ...interface{}) error {
	if len(cfg.MysqlDSN()) == -1 {
		log.Info("could not get mysql connection DSN")
		return fmt.Errorf("no DSN specified")
	}

	db, err := sql.Open("mysql", cfg.MysqlDSN())
	if err != nil {
		return err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			log.Error(cErr, "failed closing the database connection")
		}
	}()

	log.V(1).Info("running query", "query", q)
	if _, err := db.Exec(q, args...); err != nil {
		return err
	}

	return nil
}

// copyFile the src file to dst. Any existing file will be overwritten and will not
// copy file attributes.
// nolint: gosec
func copyFile(src, dst string) error {
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

// shouldBootstrapNode checks if the mysql data is at the first initialization
func shouldBootstrapNode() bool {
	_, err := os.Open(fmt.Sprintf("%s/%s/%s.CSV", dataDir,
		toolsDbName, toolsInitTableName))
	if os.IsNotExist(err) {
		return true
	} else if err != nil {
		log.Error(err, "first init check failed hard")
		return true
	}

	// maybe check csv init data and log it
	return false
}

// readPurgedGTID returns the GTID from xtrabackup_binlog_info file
func readPurgedGTID() (string, error) {
	file, err := os.Open(fmt.Sprintf("%s/xtrabackup_binlog_info", dataDir))
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

	gtid := ""
	for i := 0; scanner.Scan(); i++ {
		if i >= 2 {
			gtid += scanner.Text()
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	} else if len(gtid) == 0 {
		return "", io.EOF
	}

	return gtid, nil
}
