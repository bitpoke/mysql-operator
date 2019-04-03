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

package mysqlcluster

import (
	"fmt"

	"github.com/presslabs/controller-util/syncer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

// NewSecretSyncer returns secret syncer
// nolint: gocyclo
// TODO: this syncer is not needed anymore and can be removed in future version
func NewSecretSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, opt *options.Options) syncer.Interface {
	obj := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Spec.SecretName,
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("Secret", nil, obj, c, scheme, func(in runtime.Object) error {
		out := in.(*core.Secret)

		if _, ok := out.Data["ROOT_PASSWORD"]; !ok {
			return fmt.Errorf("ROOT_PASSWORD not set in secret: %s", out.Name)
		}

		return nil
	})
}
