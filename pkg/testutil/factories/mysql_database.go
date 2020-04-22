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

package factories

import (
	"context"
	"fmt"
	"math/rand"

	. "github.com/onsi/gomega" // nolint: golint,stylecheck

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqldatabase"
)

// MysqlDBOption to config the factory
type MysqlDBOption func(*mysqldatabase.Database) error

// WithMySQLCluster create secret and cluster and sets on the database
func WithMySQLCluster(ctx context.Context, cl client.Client, name string) MysqlDBOption {
	// create a cluster
	cluster := NewMySQLCluster(func(cluster *mysqlv1alpha1.MysqlCluster) error {
		cluster.Name = name
		return nil
	}, CreateMySQLClusterSecret(cl, &corev1.Secret{}), CreateMySQLClusterInK8s(cl))

	return func(db *mysqldatabase.Database) error {
		db.Spec.ClusterRef.Name = cluster.Name
		return nil
	}
}

// WithDBReadyCondition sets the ready status
func WithDBReadyCondition() MysqlDBOption {
	return func(db *mysqldatabase.Database) error {
		db.UpdateCondition(
			mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionTrue,
			mysqldatabase.ProvisionSucceeded, "success",
		)

		return nil
	}
}

// CreateDatabase is the option for creating project k8s resource
func CreateDatabase(ctx context.Context, c client.Client) MysqlDBOption {
	return func(db *mysqldatabase.Database) error {
		if err := c.Create(ctx, db.Unwrap()); err != nil {
			return err
		}

		return nil
	}
}

// NewDatabase returns a database object
func NewDatabase(opts ...MysqlDBOption) *mysqldatabase.Database {
	name := fmt.Sprintf("db-%d", rand.Int()) // nolint: gosec
	db := mysqldatabase.Wrap(&mysqlv1alpha1.MysqlDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: mysqlv1alpha1.MysqlDatabaseSpec{
			ClusterRef: mysqlv1alpha1.ClusterReference{
				LocalObjectReference: corev1.LocalObjectReference{Name: "does-not-exists"},
				Namespace:            "default",
			},
			Database: name,
		},
	})

	for _, opt := range opts {
		Expect(opt(db)).To(Succeed())
	}

	return db
}
