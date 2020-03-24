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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
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

	DescribeTable("defaults for innodb-buffer-pool-size and innodb-buffer-pool-instances",
		func(mem, cpu, expectedBufferSize, expectedBufferInstances string) {
			cluster = New(&api.MysqlCluster{
				Spec: api.MysqlClusterSpec{
					MysqlConf: map[string]intstr.IntOrString{},
					PodSpec: api.PodSpec{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse(cpu),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse(mem),
							},
						},
					},
				},
			})
			cluster.SetDefaults(options.GetOptions())

			valBS := cluster.Spec.MysqlConf["innodb-buffer-pool-size"]
			Expect(valBS.String()).To(Equal(expectedBufferSize))

			valBI := cluster.Spec.MysqlConf["innodb-buffer-pool-instances"]
			Expect(valBI.String()).To(Equal(expectedBufferInstances))
		},
		//        memory, cpu,  innodbBufferSize,  innodbBufferInstances
		Entry("zero", "0", "0", "0", "0"),
		Entry("< 512Mi < 1", "256Mi", "100m", "0", "1"), // innodb-buffer-pool-size not set
		Entry("< 512Mi > 1", "384Mi", "10", "0", "1"),   // innodb-buffer-pool-size not set
		Entry("> 512M", "514Mi", "2", "129M", "1"),
		Entry("< 4G", "3Gi", "2", "2112M", "2"),
		Entry("< 4G < 1", "3Gi", "100m", "2112M", "1"),
		Entry("< 8G", "5.5Gi", "6", "4096M", "4"), // (1024 * 5.5 - 512) * 0.8, instances: 4G/1G = 4
		Entry("> 8G", "12Gi", "2", "9420M", "2"),  // (1024 * 12) * 0.75
	)
})
