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

package cluster

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/test/e2e/framework"
	tutil "github.com/presslabs/mysql-operator/test/e2e/util"
)

const (
	TIMEOUT = 120 * time.Second
	POLLING = 2 * time.Second
)

var _ = Describe("Mysql cluster tests", func() {
	f := framework.NewFramework("mysql-clusters")

	It("create a cluster", func() {
		testCreateACluster(f)
	})
})

func testCreateACluster(f *framework.Framework) {
	pw := "rootPassword"
	secret := tutil.NewClusterSecret("test1", f.Namespace.Name, pw)
	_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
	Expect(err).NotTo(HaveOccurred())

	cluster := tutil.NewCluster("test1", f.Namespace.Name)
	cluster.Spec.Replicas = 1

	_, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		c, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		if err != nil {
			return false
		}
		cond := tutil.ClusterCondition(c, api.ClusterConditionReady)
		if cond == nil {
			return false
		}
		return cond.Status == core.ConditionTrue
	}, TIMEOUT, POLLING).Should(Equal(true))
}
