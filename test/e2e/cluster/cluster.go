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
	"context"
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	. "github.com/onsi/gomega/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

const (
	POLLING = 2 * time.Second
)

var (
	one = int32(1)
	two = int32(2)
)

var _ = Describe("Mysql cluster tests", func() {
	f := framework.NewFramework("mc-1")

	var (
		cluster    *api.MysqlCluster
		clusterKey types.NamespacedName
		secret     *core.Secret
		name       string
		pw         string
		err        error
	)

	BeforeEach(func() {
		// be careful, mysql allowed hostname lenght is <63
		name = fmt.Sprintf("cl-%d", rand.Int31()/1000)
		pw = fmt.Sprintf("pw-%d", rand.Int31())

		By("creating a new cluster secret")
		secret = framework.NewClusterSecret(name, f.Namespace.Name, pw)
		Expect(f.Client.Create(context.TODO(), secret)).To(Succeed(), "failed to create secret '%s", secret.Name)

		By("creating a new cluster")
		cluster = framework.NewCluster(name, f.Namespace.Name)
		clusterKey = types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		Expect(f.Client.Create(context.TODO(), cluster)).To(Succeed(), "failed to create cluster '%s'", cluster.Name)

		By("testing the cluster readiness")
		testClusterReadiness(f, cluster)

		By("testing that cluster is registered with orchestrator")
		testClusterIsRegistredWithOrchestrator(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed(), "failed to get cluster %s", cluster.Name)
	})

	It("scale up a cluster", func() {
		// scale up the cluster
		cluster.Spec.Replicas = &two
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster
		By("test cluster is ready after scale up")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after scale up")
		testClusterIsRegistredWithOrchestrator(f, cluster)
		By("test cluster endpoints are set correctly")
		testClusterEndpoints(f, cluster, []int{0}, []int{0, 1})
	})

	It("failover cluster", func() {
		cluster.Spec.Replicas = &two
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("test cluster is ready after cluster update")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster)

		// check cluster to have a master and a slave
		By("test cluster nodes master condition is properly set")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionMaster, core.ConditionTrue, f.Timeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)

		// remove master pod
		podName := framework.GetNameForResource("sts", cluster) + "-0"
		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(podName, &meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		// check failover done, this is a reggression test
		// TODO: decrease this timeout to 20
		failoverTimeout := 60 * time.Second
		By(fmt.Sprintf("Check failover done; timeout=%s", failoverTimeout))
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionTrue, failoverTimeout)

		// after some time node 0 should be up and should be slave
		By("test cluster master condition is properly set")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionReplicating, core.ConditionTrue, f.Timeout)

		By("test cluster endpoints after failover")
		testClusterEndpoints(f, cluster, []int{1}, []int{0, 1})
	})

	It("scale down a cluster", func() {
		// configure MySQL cluster to have 2 replicas and to use PV for data storage
		cluster.Spec.Replicas = &two
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("test cluster is ready after cluster update")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// check PVCs
		Eventually(f.GetClusterPVCsFn(cluster)).Should(HaveLen(2))

		// scale down the cluster
		cluster.Spec.Replicas = &one
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		By("test cluster is ready after scale down")
		testClusterReadiness(f, cluster)

		By("check pvc gets deleted")
		Eventually(f.GetClusterPVCsFn(cluster), "5s", POLLING).Should(HaveLen(1))

		// scale down the cluster to zero
		zero := int32(0)
		cluster.Spec.Replicas = &zero
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		By("test cluster is ready after scale down")
		testClusterReadiness(f, cluster)

		Consistently(f.GetClusterPVCsFn(cluster), "30s", POLLING).Should(HaveLen(1))
	})

	It("slave io running stopped", func() {
		cluster.Spec.Replicas = &two
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("test cluster is ready after cluster update")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// stop slave
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE;")

		// expect node to be removed from service and status to be updated
		By("test cluster node 1 replicating condition is set to false")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionReplicating, core.ConditionFalse, 30*time.Second)
		// node 1 should not be in healty service
		By("test cluster endpoints after stop slave")
		testClusterEndpoints(f, cluster, []int{0}, []int{0})
	})

	It("slave latency", func() {
		cluster.Spec.Replicas = &two
		one := int64(1)
		cluster.Spec.MaxSlaveLatency = &one

		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("test cluster is ready after cluster update")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after cluster update")
		testClusterIsRegistredWithOrchestrator(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// set delayed replication
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE; CHANGE MASTER TO MASTER_DELAY = 100; START SLAVE;")

		// expect node to be marked as lagged and removed from service
		By("test cluster node 1 to be lagged")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionLagged, core.ConditionTrue, 20*time.Second)
		// node 1 should not be in healty service
		By("test cluster endpoints after delayed slave")
		testClusterEndpoints(f, cluster, []int{0}, []int{0})
	})

	It("cluster readOnly", func() {
		cluster.Spec.Replicas = &two
		cluster.Spec.ReadOnly = true
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("test cluster is ready after cluster update")
		testClusterReadiness(f, cluster)
		By("test cluster is registered in orchestrator after cluster update")
		testClusterReadOnlyIsRegistredWithOrchestrator(f, cluster)

		// get cluster
		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// expect cluster to be marked readOnly
		By("test cluster to be readOnly")
		f.ClusterEventuallyCondition(cluster, api.ClusterConditionReadOnly, core.ConditionTrue, f.Timeout)

		// expect node to be marked as lagged and removed from service
		By("test cluster node 0 to be readOnly")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionReadOnly, core.ConditionTrue, 20*time.Second)

		// TODO: fix this test
		// // node 1 should not be in healthy service because is marked as lagged (heartbeat can't write to master anymore)
		// By("test cluster endpoints after delayed slave")
		// testClusterEndpoints(f, cluster, []int{0}, []int{0})

		// remove master pod
		podName := framework.GetNameForResource("sts", cluster) + "-0"
		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(podName, &meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		// check failover to not be started
		failoverTimeout := 80 * time.Second
		By(fmt.Sprintf("ensure that failover is not started; timeout=%s", failoverTimeout))
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionFalse, failoverTimeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionReadOnly, core.ConditionTrue, failoverTimeout)
	})

})

func testClusterReadiness(f *framework.Framework, cluster *api.MysqlCluster) {
	timeout := f.Timeout
	if *cluster.Spec.Replicas > 0 {
		timeout = time.Duration(*cluster.Spec.Replicas) * f.Timeout
	}

	// wait for pods to be ready
	Eventually(func() int {
		cl := &api.MysqlCluster{}
		f.Client.Get(context.TODO(), types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cl)
		return cl.Status.ReadyNodes
	}, timeout, POLLING).Should(Equal(int(*cluster.Spec.Replicas)), "Not ready replicas of cluster '%s'", cluster.Name)

	f.ClusterEventuallyCondition(cluster, api.ClusterConditionReady, core.ConditionTrue, f.Timeout)
	// TODO: investigate way sometime exists failover ACK even to a newly created cluster.
	// f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionFalse, f.Timeout)
}

//test if a non-readOnly cluster is registered with orchestrator
func testClusterIsRegistredWithOrchestrator(f *framework.Framework, cluster *api.MysqlCluster) {
	testClusterRegistrationInOrchestrator(f, cluster, false)
}

//test if a readOnly cluster is registered with orchestrator
func testClusterReadOnlyIsRegistredWithOrchestrator(f *framework.Framework, cluster *api.MysqlCluster) {
	testClusterRegistrationInOrchestrator(f, cluster, true)
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterRegistrationInOrchestrator(f *framework.Framework, cluster *api.MysqlCluster, clusterReadOnly bool) {
	Expect(f.Client.Get(context.TODO(),
		types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
		cluster)).To(Succeed())

	// update the list of expected nodes to be in orchestrator
	consistOfNodes := []GomegaMatcher{
		MatchFields(IgnoreExtras, Fields{
			"Key": Equal(orc.InstanceKey{
				Hostname: f.GetPodHostname(cluster, 0),
				Port:     3306,
			}),
			"GTIDMode":      Equal("ON"),
			"IsUpToDate":    Equal(true),
			"Binlog_format": Equal("ROW"),
			"ReadOnly":      Equal(clusterReadOnly),
		}), // master node
	}
	for i := 1; i < int(*cluster.Spec.Replicas); i++ {
		consistOfNodes = append(consistOfNodes, MatchFields(IgnoreExtras, Fields{
			"Key": Equal(orc.InstanceKey{
				Hostname: f.GetPodHostname(cluster, i),
				Port:     3306,
			}),
			"GTIDMode":      Equal("ON"),
			"IsUpToDate":    Equal(true),
			"Binlog_format": Equal("ROW"),
			"ReadOnly":      Equal(true),
		})) // slave node
	}

	// check orchestrator nodes to be equal.
	timeout := time.Duration(*cluster.Spec.Replicas) * f.Timeout
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
func testClusterEndpoints(f *framework.Framework, cluster *api.MysqlCluster, master []int, nodes []int) {
	Expect(f.Client.Get(context.TODO(),
		types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
		cluster)).To(Succeed())

	// prepare the expected list of ips that should be set in endpoints
	var masterIPs []string
	var healthyIPs []string

	for _, node := range master {
		pod := f.GetPodForNode(cluster, node)
		masterIPs = append(masterIPs, pod.Status.PodIP)
	}

	for _, node := range nodes {
		pod := f.GetPodForNode(cluster, node)
		healthyIPs = append(healthyIPs, pod.Status.PodIP)
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
	master_ep := framework.GetNameForResource("svc-master", cluster)
	if len(masterIPs) > 0 {
		Eventually(getAddrForSVC(master_ep, true), timeout).Should(ConsistOf(masterIPs), "Master ready endpoints are not correctly set.")
	} else {
		Eventually(getAddrForSVC(master_ep, true), timeout).Should(HaveLen(0), "Master ready endpoints should be 0.")
	}

	// healthy nodes service
	hnodes_ep := framework.GetNameForResource("svc-read", cluster)
	if len(healthyIPs) > 0 {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(ConsistOf(healthyIPs), "Healthy nodes ready endpoints are not correctly set.")
	} else {
		Eventually(getAddrForSVC(hnodes_ep, true), timeout).Should(HaveLen(0), "Healthy nodes not ready endpoints are not correctly set.")
	}
}
