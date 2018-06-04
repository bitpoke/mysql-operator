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

package test

import (
	"flag"
	//	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util/options"
	//	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
)

const (
	Namespace = "default"
)

func init() {

	// make tests verbose
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "5")
}

func NewFakeCluster(name string) *api.MysqlCluster {
	return &api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
		},
		Spec: api.ClusterSpec{
			Replicas:   1,
			SecretName: name,
		},
	}

}

func NewFakeBackup(name, clName string) *api.MysqlBackup {
	return &api.MysqlBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
		},
		Spec: api.BackupSpec{
			ClusterName: clName,
		},
	}
}

func NewFakeOption() *options.Options {
	opt := options.GetOptions()
	opt.Validate()
	return opt
}
