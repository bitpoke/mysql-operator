/*
Copyright 2020 Pressinfra SRL

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

package mysql

import (
	"database/sql"
)

// QueryRunner is the query runner type
type QueryRunner func(dsn string, query string, args ...interface{}) error

// StandardQueryRunner executes a MySQL query
func StandardQueryRunner(dsn, q string, args ...interface{}) error {
	db, err := sql.Open("mysql", dsn)
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
