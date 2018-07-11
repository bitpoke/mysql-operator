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
	"fmt"
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
		By("Create a cluster")
		pw := "rootPassword"
		secret := tutil.NewClusterSecret("test1", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		cluster := tutil.NewCluster("test1", f.Namespace.Name)
		cluster.Spec.Replicas = 1

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		testCreateACluster(f, cluster, "after cluster creation")
		testClusterIsInOrchestartor(f, cluster, "after cluster creation")
		testClusterEndpoints(f, cluster, []int{0}, []int{0}, "after cluster creation")
	})

	It("scale up a cluster", func() {
		pw := "rootPassword2"
		secret := tutil.NewClusterSecret("scale-up", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		cluster := tutil.NewCluster("scale-up", f.Namespace.Name)
		cluster.Spec.Replicas = 1

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		testCreateACluster(f, cluster, "after cluster creation")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		// scale up the cluster
		cluster.Spec.Replicas = 2
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: %s", cluster.Name)

		testCreateACluster(f, cluster, "after scale up")
		testClusterIsInOrchestartor(f, cluster, "after scale up")
		testClusterEndpoints(f, cluster, []int{0}, []int{0, 1}, "after scale up")
	})

	It("failover cluster", func() {
		pw := "rootPassword3"
		secret := tutil.NewClusterSecret("failover", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		cluster := tutil.NewCluster("failover", f.Namespace.Name)
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		testCreateACluster(f, cluster, "after cluster creation")
		testClusterIsInOrchestartor(f, cluster, "after cluster creation")

		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionTrue)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionFalse)

		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete("failover-mysql-0", &meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", "failover-mysql-0")

		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionUnknown)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionTrue)

		// after some time node 0 should be up and should be slave
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionFalse)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionReplicating, core.ConditionTrue)

		testClusterEndpoints(f, cluster, []int{1}, []int{0, 1}, "after failover")
	})

	It("scale down a cluster", func() {
		pw := "rootPassword4"
		secret := tutil.NewClusterSecret("scale-down", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		cluster := tutil.NewCluster("scale-down", f.Namespace.Name)
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		testCreateACluster(f, cluster, "after cluster creation")
		testClusterIsInOrchestartor(f, cluster, "after cluster creation")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		// scale down the cluster
		cluster.Spec.Replicas = 1
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: %s", cluster.Name)

		testCreateACluster(f, cluster, "after scale down")
		//testCreateAClusterWithPendingAck(f, cluster)
	})

	It("slave latency", func() {
		pw := "rootPassword6"
		secret := tutil.NewClusterSecret("latency", f.Namespace.Name, pw)
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		cluster := tutil.NewCluster("latency", f.Namespace.Name)
		cluster.Spec.Replicas = 2
		one := int64(1)
		cluster.Spec.MaxSlaveLatency = &one

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		testCreateACluster(f, cluster, "after cluster creation")
		testClusterIsInOrchestartor(f, cluster, "after cluster creation")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE;")

		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionReplicating, core.ConditionFalse)
		// node 1 should not be in healty service
		testClusterEndpoints(f, cluster, []int{0}, []int{0}, "after stop slave")
	})

})

func testCreateACluster(f *framework.Framework, cluster *api.MysqlCluster, where string) {
	By(fmt.Sprintf("Test cluster is ready: %s", where))
	Eventually(func() int {
		cluster, _ = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		return cluster.Status.ReadyNodes
	}, TIMEOUT, POLLING).Should(Equal(int(cluster.Spec.Replicas)), "Not ready replicas of cluster '%s'", cluster.Name)

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue)
	f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionFalse)
}

func testCreateAClusterWithPendingAck(f *framework.Framework, cluster *api.MysqlCluster, where string) {
	By(fmt.Sprintf("Test cluster is ready: %s", where))
	Eventually(func() int {
		cluster, _ := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		return cluster.Status.ReadyNodes
	}, TIMEOUT, POLLING).Should(Equal(int(cluster.Spec.Replicas)), "Not ready replicas of cluster '%s'", cluster.Name)

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue)
	f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionTrue)
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterIsInOrchestartor(f *framework.Framework, cluster *api.MysqlCluster, where string) {
	By(fmt.Sprintf("Test cluster is in orchestrator: %s", where))
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to get cluster '%s'", cluster.Name)

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

	}, TIMEOUT, POLLING).Should(ConsistOf(consistOfNodes), "Cluster is not configured correctly in orchestrator.")
}

// checks for cluster endpoints to exists when cluster is ready
// TODO: check in more detail
func testClusterEndpoints(f *framework.Framework, cluster *api.MysqlCluster, master []int, nodes []int, where string) {
	By(fmt.Sprintf("Test cluster endpoints are configured corectly: %s", where))
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to get cluster: '%s'", cluster.Name)

	var masterIPs []string
	var healtyIPs []string

	for _, node := range master {
		pod := f.GetPodForNode(cluster, node)
		masterIPs = append(masterIPs, pod.Status.PodIP)
	}

	for _, node := range nodes {
		pod := f.GetPodForNode(cluster, node)
		healtyIPs = append(healtyIPs, pod.Status.PodIP)
	}

	getAddrForSVC := func(name string, ready bool) func() []string {
		return func() []string {
			endpoints, err := f.ClientSet.CoreV1().Endpoints(cluster.Namespace).Get(name, meta.GetOptions{})
			if err != nil {
				return nil
			}

			addrs := endpoints.Subsets[0].NotReadyAddresses
			if ready {
				addrs = endpoints.Subsets[0].Addresses
			}

			var ips []string
			for _, addr := range addrs {
				ips = append(ips, addr.IP)
			}

			return ips
		}
	}

	timeout := 30 * time.Second

	// master service
	master_ep := cluster.GetNameForResource(api.MasterService)
	if len(masterIPs) > 0 {
		Eventually(getAddrForSVC(master_ep, true), timeout).Should(ConsistOf(masterIPs), "Master ready endpoints are not correctly set.")
	} else {
		Eventually(getAddrForSVC(master_ep, true), timeout).Should(HaveLen(0), "Master ready endpoints should be 0.")
	}
	//Eventually(getAddrForSVC(master_ep, false), timeout).Should(HaveLen(0), "Master not ready endpoints should be 0.")

	// healty nodes service
	hnodes_ep := cluster.GetNameForResource(api.HealthyNodesService)
	if len(healtyIPs) > 0 {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(ConsistOf(healtyIPs), "Healty nodes ready endpoints are not correctly set.")
	} else {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(HaveLen(0), "Healty nodes not ready endpoints are not correctly set.")
	}
	//Eventually(getAddrForSVC(hnodes_ep, false), timeout).Should(
	//	HaveLen(int(cluster.Spec.Replicas) - cluster.Status.ReadyNodes))
}
