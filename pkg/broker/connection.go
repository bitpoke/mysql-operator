/*
Copyright 2019 Pressinfra SRL

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

package broker

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
	"strings"
)

type connection interface {
	Open() error
	Close()
	RunQueries(ctx context.Context, queries []string) error
}

type mysqlConn struct {
	dsn string
	db  *sql.DB
}

func newConnection(cluster *mysqlcluster.MysqlCluster, user, password string) connection {
	host := fmt.Sprintf("%s.%s", cluster.GetNameForResource(mysqlcluster.MasterService), cluster.Namespace)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		user, password, host, constants.MysqlPort,
	)

	return &mysqlConn{
		dsn: dsn,
	}
}

func (mc *mysqlConn) Open() error {
	db, err := sql.Open("mysql", mc.dsn)
	if err != nil {
		return err
	}

	mc.db = db
	return nil
}

func (mc *mysqlConn) Close() {
	if err := mc.db.Close(); err != nil {
		log.Error(err, "mysql can't close connection")
	}
}

func (mc *mysqlConn) RunQueries(ctx context.Context, queries []string) error {
	q := strings.Join(queries, ";")

	_, err := mc.db.ExecContext(ctx, q)
	return err
}
