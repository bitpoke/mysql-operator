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
	. "github.com/onsi/gomega/gstruct"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
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
		cluster := testCreateACluster(f)
		testClusterIsInOrchestartor(f, cluster)
		testClusterEndpoints(f, cluster)
	})

})

func testCreateACluster(f *framework.Framework) *api.MysqlCluster {
	pw := "rootPassword"
	secret := tutil.NewClusterSecret("test1", f.Namespace.Name, pw)
	_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
	Expect(err).NotTo(HaveOccurred())

	cluster := tutil.NewCluster("test1", f.Namespace.Name)
	cluster.Spec.Replicas = 1

	cl, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
	Expect(err).NotTo(HaveOccurred())

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue)

	return cl
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterIsInOrchestartor(f *framework.Framework, cluster *api.MysqlCluster) {
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() []orc.Instance {
		insts, err := f.OrcClient.Cluster(tutil.OrcClusterName(cluster))
		if err != nil {
			return nil
		}

		return insts

	}, TIMEOUT, POLLING).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
		"Key": Equal(orc.InstanceKey{
			Hostname: cluster.GetPodHostname(0),
			Port:     3306,
		}),
		"GTIDMode":      Equal("ON"),
		"IsUpToDate":    Equal(true),
		"Binlog_format": Equal("ROW"),
		"ReadOnly":      Equal(false),
	})))
}

// checks for cluster endpoints to exists when cluster is ready
func testClusterEndpoints(f *framework.Framework, cluster *api.MysqlCluster) {
	// master service
	master_ep := cluster.GetNameForResource(api.MasterService)
	endpoints, err := f.ClientSet.CoreV1().Endpoints(cluster.Namespace).Get(master_ep, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(endpoints.Subsets[0].Addresses).To(HaveLen(1))
	Expect(endpoints.Subsets[0].NotReadyAddresses).To(HaveLen(0))

	// healty nodes service
	hnodes_ep := cluster.GetNameForResource(api.HealthyNodesService)
	endpoints, err = f.ClientSet.CoreV1().Endpoints(cluster.Namespace).Get(hnodes_ep, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(endpoints.Subsets[0].Addresses).To(HaveLen(cluster.Status.ReadyNodes))
	Expect(endpoints.Subsets[0].NotReadyAddresses).To(HaveLen(int(cluster.Spec.Replicas) - cluster.Status.ReadyNodes))
}
