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
	User        string
	Password    string
	Host        string
	Port        int32
	queryRunner QueryRunner
}

// NewConfigFromClusterKey returns a new Config based on a MySQLCluster key
func NewConfigFromClusterKey(c client.Client, clusterKey client.ObjectKey, qr QueryRunner) (*Config, error) {
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
		User:        "root",
		Password:    string(secret.Data["ROOT_PASSWORD"]),
		Host:        fmt.Sprintf("%s-mysql-master.%s", cluster.Name, cluster.Namespace),
		Port:        3306,
		queryRunner: qr,
	}, nil
}

// GetMysqlDSN returns a data source name
func (c *Config) GetMysqlDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port,
	)
}

// RunQuery runs the given query through the config's queryRunner
func (c *Config) RunQuery(q string, args ...interface{}) error {
	return c.queryRunner(c.GetMysqlDSN(), q, args...)
}
