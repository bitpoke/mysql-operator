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

	. "github.com/onsi/gomega" // nolint: golint, stylecheck

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqluser"
)

// MySQLUserOption is the option type for for the invite factory
type MySQLUserOption func(*mysqluser.MySQLUser) error

// BuildMySQLUser is a helper func that builds a mysql user
func BuildMySQLUser(cl client.Client, cluster *mysqlv1alpha1.MysqlCluster, opts ...MySQLUserOption) *mysqluser.MySQLUser {
	// Set a default user and implicitly a resource name
	user := fmt.Sprintf("user-%d", rand.Int31())
	opts = append([]MySQLUserOption{WithUser(user)}, opts...)

	mu := mysqluser.Wrap(&mysqlv1alpha1.MySQLUser{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: mysqlv1alpha1.MySQLUserSpec{
			ClusterRef: mysqlv1alpha1.ClusterReference{
				LocalObjectReference: corev1.LocalObjectReference{Name: cluster.Name},
				Namespace:            cluster.Namespace,
			},
			AllowedHosts: []string{"%"},
		},
	})

	for _, opt := range opts {
		Expect(opt(mu)).To(Succeed())
	}

	return mu
}

// CreateMySQLUser is a helper func that builds a mysql user
func CreateMySQLUser(cl client.Client, cluster *mysqlv1alpha1.MysqlCluster, opts ...MySQLUserOption) *mysqluser.MySQLUser {
	mu := BuildMySQLUser(cl, cluster, opts...)
	Expect(cl.Create(context.TODO(), mu.Unwrap())).To(Succeed())

	return mu
}

// WithUser is an option to specify a user when creating the MySQLUser
func WithUser(user string) MySQLUserOption {
	return func(mu *mysqluser.MySQLUser) error {
		mu.ObjectMeta.Name = user //mysqluser.UserToName(user)
		mu.Spec.User = user

		return nil
	}
}

// WithPassword is an option to specify a password when creating the MySQLUser
func WithPassword(cl client.Client, password string) MySQLUserOption {
	return func(mu *mysqluser.MySQLUser) error {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mu.Name,
				Namespace: mu.Namespace,
			},
			Data: map[string][]byte{
				"password": []byte(password),
			},
		}

		Expect(cl.Create(context.TODO(), secret))

		mu.Spec.Password = corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
			Key:                  "password",
		}

		return nil
	}
}

// WithPermissions is an option to specify user permissions when creating the MySQLUser
func WithPermissions(permissions ...mysqlv1alpha1.MySQLPermission) MySQLUserOption {
	return func(mu *mysqluser.MySQLUser) error {
		mu.Spec.Permissions = append(mu.Spec.Permissions, permissions...)

		return nil
	}
}

// WithUserReadyCondition sets the ready status
func WithUserReadyCondition() MySQLUserOption {
	return func(mu *mysqluser.MySQLUser) error {
		mu.UpdateStatusCondition(
			mysqlv1alpha1.MySQLUserReady, corev1.ConditionTrue,
			"ProvisionSucceeded", "success",
		)

		return nil
	}
}
