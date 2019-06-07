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

package mysqlcluster

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	logf "github.com/presslabs/controller-util/log"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

func TestMySQLClusterWrapper(t *testing.T) {
	logf.SetLogger(logf.ZapLoggerTo(GinkgoWriter, true))

	RegisterFailHandler(Fail)
	RunSpecs(t, "Sidecar App Suite")
}

var _ = Describe("Test MySQL cluster wrapper", func() {
	var (
		cluster *MysqlCluster
	)

	BeforeEach(func() {
		cluster = New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cl-name",
				Namespace: "default",
			},
			Spec: api.MysqlClusterSpec{
				SecretName: "sct-name",
				MysqlConf:  map[string]intstr.IntOrString{},
			},
		})
		// set defaults
		api.SetDefaults_MysqlCluster(cluster.Unwrap())
		cluster.SetDefaults(options.GetOptions())
	})

	It("should have defaults set", func() {
		Expect(cluster.GetMySQLSemVer()).To(Equal(constants.MySQLDefaultVersion))
		Expect(cluster.GetMysqlImage()).To(ContainSubstring("percona"))

		Expect(cluster.Spec.PodSpec.Resources.Requests.Memory()).To(PointTo(Equal(resource.MustParse("1Gi"))))
		Expect(cluster.Spec.MysqlConf).To(HaveKey(Equal("innodb-buffer-pool-size")))
		Expect(cluster.Spec.MysqlConf).To(HaveKey(Equal("innodb-log-file-size")))
		Expect(cluster.Spec.MysqlConf).NotTo(HaveKey(Equal("max-binlog-size")))
	})

	It("should use init MySQL container", func() {
		Expect(cluster.ShouldHaveInitContainerForMysql()).To(Equal(true))
	})

	It("should return 0.0.0 version if wrong mysql version was given", func() {
		cluster.Spec.MysqlVersion = "wrong"

		Expect(cluster.GetMySQLSemVer().String()).To(Equal("0.0.0"))
		// no image for version 0.0.0
		Expect(cluster.GetMysqlImage()).To(Equal(""))
		// should not use mysql init only container
		Expect(cluster.ShouldHaveInitContainerForMysql()).To(Equal(false))
	})

	It("should return the given version", func() {
		version := "5.7.24"
		cluster.Spec.MysqlVersion = version

		Expect(cluster.GetMySQLSemVer().String()).To(Equal(version))
		Expect(cluster.GetMysqlImage()).To(Equal(constants.MysqlImageVersions[version]))
		// should not use mysql init only container
		Expect(cluster.ShouldHaveInitContainerForMysql()).To(Equal(false))
	})
})
