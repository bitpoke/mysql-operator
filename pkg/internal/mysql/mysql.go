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
	"context"
	"database/sql"
	"fmt"
	// this import  needs to be done otherwise the mysql driver don't work
	_ "github.com/go-sql-driver/mysql"
	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	logf "github.com/presslabs/controller-util/log"
)

var log = logf.Log.WithName("mysql-internal")

// Config is used to connect to a MysqlCluster
type Config struct {
	User     string
	Password string
	Host     string
	Port     int32
}

// NewConfigFromClusterKey returns a new Config based on a MySQLCluster key
func NewConfigFromClusterKey(c client.Client, clusterKey client.ObjectKey) (*Config, error) {
	cluster := &mysqlv1alpha1.MysqlCluster{}
	if err := c.Get(context.TODO(), clusterKey, cluster); err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{Name: cluster.Spec.SecretName, Namespace: cluster.Namespace}

	if err := c.Get(context.TODO(), secretKey, secret); err != nil {
		return nil, err
	}

	return &Config{
		User:     "root",
		Password: string(secret.Data["ROOT_PASSWORD"]),
		Host:     fmt.Sprintf("%s-mysql-master.%s", cluster.Name, cluster.Namespace),
		Port:     3306,
	}, nil
}

// GetMysqlDSN returns a data source name
func (c *Config) GetMysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port,
	)
}

// Rows interface is a subset of mysql.Rows
type Rows interface {
	Err() error
	Next() bool
	Scan(dest ...interface{}) error
}

// SQLRunner interface is a subset of mysql.DB
type SQLRunner interface {
	QueryExec(ctx context.Context, query Query) error
	QueryRow(ctx context.Context, query Query, dest ...interface{}) error
	QueryRows(ctx context.Context, query Query) (Rows, error)
}

type sqlRunner struct {
	db *sql.DB
}

// SQLRunnerFactory a function that generates a new SQLRunner
type SQLRunnerFactory func(cfg *Config, errs ...error) (SQLRunner, func(), error)

// NewSQLRunner opens a connections using the given DSN
func NewSQLRunner(cfg *Config, errs ...error) (SQLRunner, func(), error) {
	var db *sql.DB
	var close func()

	// make this factory accept a functions that tries to generate a config
	if len(errs) > 0 && errs[0] != nil {
		return nil, close, errs[0]
	}

	db, err := sql.Open("mysql", cfg.GetMysqlDSN())
	if err != nil {
		return nil, close, err
	}

	// close connection function
	close = func() {
		if cErr := db.Close(); cErr != nil {
			log.Error(cErr, "failed closing the database connection")
		}
	}

	return &sqlRunner{db: db}, close, nil
}

func (sr sqlRunner) QueryExec(ctx context.Context, query Query) error {
	_, err := sr.db.ExecContext(ctx, query.escapedQuery, query.args...)
	return err
}
func (sr sqlRunner) QueryRow(ctx context.Context, query Query, dest ...interface{}) error {
	return sr.db.QueryRowContext(ctx, query.escapedQuery, query.args...).Scan(dest...)
}
func (sr sqlRunner) QueryRows(ctx context.Context, query Query) (Rows, error) {
	rows, err := sr.db.QueryContext(ctx, query.escapedQuery, query.args...)
	if err != nil {
		return nil, err
	}

	return rows, rows.Err()
}
