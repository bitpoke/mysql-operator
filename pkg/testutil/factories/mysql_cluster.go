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
	"errors"
	"fmt"
	"math/rand"

	. "github.com/onsi/gomega" // nolint: golint, stylecheck
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

// MySQLClusterOption is the option type for for the invite factory
type MySQLClusterOption func(*mysqlv1alpha1.MysqlCluster) error

// CreateMySQLClusterInK8s is the option for creating mysql cluster k8s resource
func CreateMySQLClusterInK8s(c client.Client) MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		if err := c.Create(context.TODO(), mc); err != nil {
			return err
		}

		return c.Status().Update(context.TODO(), mc)
	}
}

// CreateMySQLClusterSecret is the option for creating mysql cluster secret
func CreateMySQLClusterSecret(c client.Client, secret *corev1.Secret) MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		if secret == nil {
			return errors.New("secret may not be nil")
		}

		if secret.ObjectMeta.Name == "" {
			secret.ObjectMeta.Name = mc.Spec.SecretName
			secret.ObjectMeta.Namespace = mc.ObjectMeta.Namespace
		}

		secret.Type = corev1.SecretTypeOpaque
		secret.StringData = map[string]string{
			"ROOT_PASSWORD": "password",
		}

		return c.Create(context.TODO(), secret)
	}
}

// WithMySQLClusterReadyPods sets the reay nodes status
func WithMySQLClusterReadyPods(ctx context.Context, c client.StatusClient, readyNodes int) MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		mc.Status.ReadyNodes = readyNodes

		return c.Status().Update(ctx, mc)
	}
}

// WithClusterReadyCondition sets the ready status to true
func WithClusterReadyCondition() MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		mcWrap := mysqlcluster.New(mc)
		mcWrap.UpdateStatusCondition(
			mysqlv1alpha1.ClusterConditionReady, corev1.ConditionTrue,
			"StatefulSetReady", "Success",
		)

		return nil
	}
}

// WithClusterNotReadyCondition sets the ready status to false
func WithClusterNotReadyCondition() MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		mcWrap := mysqlcluster.New(mc)
		mcWrap.UpdateStatusCondition(
			mysqlv1alpha1.ClusterConditionReady, corev1.ConditionFalse,
			"StatefulSetNotReady", "It no work",
		)

		return nil
	}
}

// WithClusterAffinity sets affinity for mysql cluster
func WithClusterAffinity(affinity *corev1.Affinity) MySQLClusterOption {
	return func(mc *mysqlv1alpha1.MysqlCluster) error {
		mc.Spec.PodSpec.Affinity = affinity

		return nil
	}
}

// NewMySQLCluster is a helper func that creates a mysql cluster
func NewMySQLCluster(opts ...MySQLClusterOption) *mysqlv1alpha1.MysqlCluster {
	name := fmt.Sprintf("cluster-%d", rand.Int31())

	mc := &mysqlv1alpha1.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			Labels:    map[string]string{},
		},

		Spec: mysqlv1alpha1.MysqlClusterSpec{
			SecretName: name,
			PodSpec: mysqlv1alpha1.PodSpec{
				Resources: corev1.ResourceRequirements{
					Limits:   corev1.ResourceList{},
					Requests: corev1.ResourceList{},
				},
			},
			VolumeSpec: mysqlv1alpha1.VolumeSpec{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{},
						Requests: corev1.ResourceList{},
					},
				},
			},
		},
	}

	for _, opt := range opts {
		Expect(opt(mc)).To(Succeed())
	}

	return mc
}
