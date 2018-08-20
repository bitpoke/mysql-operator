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
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	. "github.com/onsi/gomega/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

const (
	POLLING = 2 * time.Second
)

var _ = Describe("Mysql cluster tests", func() {
	f := framework.NewFramework("mc-1")

	var (
		cluster *api.MysqlCluster
		secret  *core.Secret
		name    string
		pw      string
		err     error
	)

	BeforeEach(func() {
		// be careful, mysql allowed hostname lenght is <63
		name = fmt.Sprintf("cl-%d", rand.Int31()/1000)

		By("creating a new cluster secret")
		pw = fmt.Sprintf("pw-%d", rand.Int31())
		secret = framework.NewClusterSecret(name, f.Namespace.Name, pw)
		_, err = f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(secret)
		Expect(err).ToNot(HaveOccurred(), "Failed to create secret '%s'", secret.Name)

		By("creating a new cluster")
		cluster = framework.NewCluster(name, f.Namespace.Name)
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Create(cluster)
		Expect(err).ToNot(HaveOccurred(), "Failed to create cluster: '%s'", cluster.Name)

		By("testing the cluster readiness")
		testClusterReadiness(f, cluster, "after creation")

		By("testing that cluster is registered with orchestrator")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after creation")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).ToNot(HaveOccurred(), "Failed to get cluster %s", cluster.Name)
	})

	It("scale up a cluster", func() {
		// scale up the cluster
		cluster.Spec.Replicas = 2
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: %s", cluster.Name)

		// test cluster
		testClusterReadiness(f, cluster, "after scale up")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after scale up")
		testClusterEndpoints(f, cluster, []int{0}, []int{0, 1}, "after scale up")
	})

	It("failover cluster", func() {
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: '%s'", cluster.Name)

		// test cluster to be ready
		testClusterReadiness(f, cluster, "after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after cluster update")

		// check cluster to have a master and a slave
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionTrue, f.Timeout)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)

		// remove master pod
		podName := cluster.GetNameForResource(api.StatefulSet) + "-0"
		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(podName, &meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		// check failover done, this is a reggression test
		// TODO: decrease this timeout to 20
		failoverTimeout := 40 * time.Second
		By(fmt.Sprintf("Check failover done; timeout=%s", failoverTimeout))
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionMaster, core.ConditionTrue, failoverTimeout)

		// after some time node 0 should be up and should be slave
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(0), api.NodeConditionReplicating, core.ConditionTrue, f.Timeout)

		testClusterEndpoints(f, cluster, []int{1}, []int{0, 1}, "after failover")
	})

	It("scale down a cluster", func() {
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: '%s'", cluster.Name)

		// test cluster to be ready
		testClusterReadiness(f, cluster, "after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after cluster update")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		// scale down the cluster
		cluster.Spec.Replicas = 1
		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: %s", cluster.Name)

		testClusterReadiness(f, cluster, "after scale down")

		// TODO: check for PVCs
	})

	It("slave io running stopped", func() {
		cluster.Spec.Replicas = 2

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: '%s'", cluster.Name)

		// test cluster to be ready
		testClusterReadiness(f, cluster, "after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after cluster update")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		// stop slave
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE;")

		// expect node to be removed from service and status to be updated
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionReplicating, core.ConditionFalse, 30*time.Second)
		// node 1 should not be in healty service
		testClusterEndpoints(f, cluster, []int{0}, []int{0}, "after stop slave")
	})

	It("slave latency", func() {
		cluster.Spec.Replicas = 2
		one := int64(1)
		cluster.Spec.MaxSlaveLatency = &one

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Update(cluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to update cluster: '%s'", cluster.Name)

		// test cluster to be ready
		testClusterReadiness(f, cluster, "after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster, "after cluster update")

		cluster, err = f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to get cluster %s", cluster.Name)

		// set delayed replication
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE; CHANGE MASTER TO MASTER_DELAY = 100; START SLAVE;")

		// expect node to be marked as lagged and removed from service
		f.NodeEventuallyCondition(cluster, cluster.GetPodHostname(1), api.NodeConditionLagged, core.ConditionTrue, 20*time.Second)
		// node 1 should not be in healty service
		testClusterEndpoints(f, cluster, []int{0}, []int{0}, "after delayed slave")
	})

})

func testClusterReadiness(f *framework.Framework, cluster *api.MysqlCluster, where string) {
	timeout := time.Duration(cluster.Spec.Replicas) * f.Timeout
	By(fmt.Sprintf("Test cluster is ready: %s; timeout=%v", where, timeout))

	// wait for pods to be ready
	Eventually(func() int {
		cluster, _ := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
		return cluster.Status.ReadyNodes
	}, timeout, POLLING).Should(Equal(int(cluster.Spec.Replicas)), "Not ready replicas of cluster '%s'", cluster.Name)

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue, f.Timeout)
	// TODO: investigate way sometime exists failover ACK even to a newly created cluster.
	// f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionFalse, f.Timeout)
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterIsRegistredWithOrchestrator(f *framework.Framework, cluster *api.MysqlCluster, where string) {
	By(fmt.Sprintf("Test cluster is in orchestrator: %s", where))
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to get cluster '%s'", cluster.Name)

	// update the list of expected nodes to be in orchestrator
	consistOfNodes := []GomegaMatcher{
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

	// check orchestrator nodes to be equal.
	timeout := time.Duration(cluster.Spec.Replicas) * f.Timeout
	Eventually(func() []orc.Instance {
		insts, err := f.OrcClient.Cluster(framework.OrcClusterName(cluster))
		if err != nil {
			return nil
		}

		return insts

	}, timeout, POLLING).Should(ConsistOf(consistOfNodes), "Cluster is not configured correctly in orchestrator.")
}

// checks for cluster endpoints to exists when cluster is ready
// TODO: check in more detail
func testClusterEndpoints(f *framework.Framework, cluster *api.MysqlCluster, master []int, nodes []int, where string) {
	By(fmt.Sprintf("Test cluster endpoints are configured corectly: %s", where))
	cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(cluster.Name, meta.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to get cluster: '%s'", cluster.Name)

	// preper the expected list of ips that should be set in endpoints
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

	// a helper function that return a callback that returns ips for a specific service
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

	// healty nodes service
	hnodes_ep := cluster.GetNameForResource(api.HealthyNodesService)
	if len(healtyIPs) > 0 {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(ConsistOf(healtyIPs), "Healty nodes ready endpoints are not correctly set.")
	} else {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(HaveLen(0), "Healty nodes not ready endpoints are not correctly set.")
	}
}
