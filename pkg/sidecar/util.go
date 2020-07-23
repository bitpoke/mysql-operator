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
	"fmt"
	"io"
	"os"

	// add mysql driver
	_ "github.com/go-sql-driver/mysql"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("sidecar")

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
