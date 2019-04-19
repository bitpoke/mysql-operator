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
)

// const (
// 	// timeOut represents the number of tries to check mysql to be ready.
// 	timeOut = 60
// 	// connRetry represents the number of tries to connect to master server
// 	connRetry = 10
// )

// RunSidecarCommand is the main command, and represents the runtime helper that
// configures the mysql server
func RunSidecarCommand(cfg *Config, stop <-chan struct{}) error {
	// TODO: don't create init table in init-files
	log.Info("doing bootstrap")
	if gtid, err := readPurgedGTID(); err == nil {
		log.Info("RESET MASTER and setting GTID_PURGED", "gtid", gtid)
		if errQ := runQuery(cfg, "RESET MASTER; SET GLOBAL GTID_PURGED=?", gtid); errQ != nil {
			return errQ
		}

		// remove the file to avoid setting GTID_PURGED next time.
		if errR := os.Remove(fmt.Sprintf("%s/xtrabackup_binlog_info", dataDir)); errR != nil {
			return errR
		}
	} else {
		log.V(-1).Info("can't determine what GTID to purge", "error", err)
	}

	log.Info("start http server for backups")
	srv := newServer(cfg, stop)
	return srv.ListenAndServe()
}

// func configTopology(cfg *Config) error {
// 	if cfg.NodeRole() == SlaveNode {
// 		log.Info("setting up as slave")
// 		if shouldBootstrapNode() {
// 			log.Info("doing bootstrap")
// 			if gtid, err := readPurgedGTID(); err == nil {
// 				log.Info("RESET MASTER and setting GTID_PURGED", "gtid", gtid)
// 				if errQ := runQuery(cfg, "RESET MASTER; SET GLOBAL GTID_PURGED=?", gtid); errQ != nil {
// 					return errQ
// 				}
// 			} else {
// 				log.V(-1).Info("can't determine what GTID to purge", "error", err)
// 			}
// 		}
//
// 		// slave node
// 		query := `
// 		  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
// 		    MASTER_HOST=?,
// 		    MASTER_USER=?,
// 		    MASTER_PASSWORD=?,
// 		    MASTER_CONNECT_RETRY=?;
// 		`
// 		if err := runQuery(cfg, query,
// 			cfg.MasterFQDN(), cfg.ReplicationUser, cfg.ReplicationPassword, connRetry,
// 		); err != nil {
// 			return fmt.Errorf("failed to configure slave node, err: %s", err)
// 		}
//
// 		query = "START SLAVE;"
// 		if err := runQuery(cfg, query); err != nil {
// 			log.Info("failed to start slave in the simple mode, trying a second method")
// 			// TODO: https://bugs.mysql.com/bug.php?id=83713
// 			query2 := `
// 			  reset slave;
// 			  start slave IO_THREAD;
// 			  stop slave IO_THREAD;
// 			  reset slave;
// 			  start slave;
// 			`
// 			if err := runQuery(cfg, query2); err != nil {
// 				return fmt.Errorf("failed to start slave node, err: %s", err)
// 			}
// 		}
// 	}
//
// 	return nil
// }

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
