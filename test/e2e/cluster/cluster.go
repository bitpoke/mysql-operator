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
	gtypes "github.com/onsi/gomega/types"
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
	f := framework.NewFramework("mc-1")

	It("create a cluster", func() {
		pw := "rootPassword"
		secret := tutil.NewClusterSecret("test1", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred())

		cluster := tutil.NewCluster("test1", f.Namespace.Name)
		cluster.Spec.Replicas = 1

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred())

		testCreateACluster(f, cluster)
		testClusterIsInOrchestartor(f, cluster)
		testClusterEndpoints(f, cluster)
	})

	It("scale cluster up", func() {
		pw := "rootPassword2"
		secret := tutil.NewClusterSecret("scale-up", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred())

		cluster := tutil.NewCluster("scale-up", f.Namespace.Name)
		cluster.Spec.Replicas = 1

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred())

		testCreateACluster(f, cluster)

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		// scale up the cluster
		cluster.Spec.Replicas = 2
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred())

		testCreateACluster(f, cluster)
		testClusterIsInOrchestartor(f, cluster)
		testClusterEndpoints(f, cluster)
	})

	It("failover cluster", func() {
		pw := "rootPassword3"
		secret := tutil.NewClusterSecret("failover", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred())

		cluster := tutil.NewCluster("failover", f.Namespace.Name)
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred())

		testCreateACluster(f, cluster)
		testClusterIsInOrchestartor(f, cluster)

		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionTrue)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionFalse)

		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete("failover-mysql-0", &meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionUnknown)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionTrue)
		testClusterEndpoints(f, cluster)
	})

})

func testCreateACluster(f *framework.Framework, cluster *api.MysqlCluster) {
	Eventually(func() int {
		cluster, _ := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		return cluster.Status.ReadyNodes
	}, TIMEOUT, POLLING).Should(Equal(int(cluster.Spec.Replicas)))

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue)
	f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionFalse)
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterIsInOrchestartor(f *framework.Framework, cluster *api.MysqlCluster) {
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	consistOfNodes := []gtypes.GomegaMatcher{
		MatchFields(IgnoreExtras, Fields{
			"Key": Equal(orc.InstanceKey{
				Hostname: cluster.GetPodHostname(0),
				Port:     3306,
			}),
			"GTIDMode":      Equal("ON"),
			"IsUpToDate":    Equal(true),
			"Binlog_format": Equal("ROW"),
			"ReadOnly":      Equal(false),
		}), // master node
	}
	for i := 1; i < int(cluster.Spec.Replicas); i++ {
		consistOfNodes = append(consistOfNodes, MatchFields(IgnoreExtras, Fields{
			"Key": Equal(orc.InstanceKey{
				Hostname: cluster.GetPodHostname(i),
				Port:     3306,
			}),
			"GTIDMode":      Equal("ON"),
			"IsUpToDate":    Equal(true),
			"Binlog_format": Equal("ROW"),
			"ReadOnly":      Equal(true),
		})) // slave node
	}

	Eventually(func() []orc.Instance {
		insts, err := f.OrcClient.Cluster(tutil.OrcClusterName(cluster))
		if err != nil {
			return nil
		}

		return insts

	}, TIMEOUT, POLLING).Should(ConsistOf(consistOfNodes))
}

// checks for cluster endpoints to exists when cluster is ready
// TODO: check in more detail
func testClusterEndpoints(f *framework.Framework, cluster *api.MysqlCluster) {
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	getAddrForSVC := func(name string, ready bool) func() []core.EndpointAddress {
		return func() []core.EndpointAddress {
			endpoints, err := f.ClientSet.CoreV1().Endpoints(cluster.Namespace).Get(name, meta.GetOptions{})
			if err != nil {
				return nil
			}

			if ready {
				return endpoints.Subsets[0].Addresses
			}
			return endpoints.Subsets[0].NotReadyAddresses
		}
	}

	timeout := 10 * time.Second

	// master service
	master_ep := cluster.GetNameForResource(api.MasterService)
	Eventually(getAddrForSVC(master_ep, true), timeout).Should(HaveLen(1))
	Eventually(getAddrForSVC(master_ep, false), timeout).Should(HaveLen(0))

	// healty nodes service
	hnodes_ep := cluster.GetNameForResource(api.HealthyNodesService)
	Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(HaveLen(cluster.Status.ReadyNodes))
	Eventually(getAddrForSVC(hnodes_ep, false), timeout).Should(
		HaveLen(int(cluster.Spec.Replicas) - cluster.Status.ReadyNodes))
}
