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
	gomegatypes "github.com/onsi/gomega/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/bitpoke/mysql-operator/pkg/orchestrator"
	"github.com/bitpoke/mysql-operator/test/e2e/framework"
)

const (
	POLLING = 500 * time.Millisecond
)

var (
	one = int32(1)
	two = int32(2)
)

var _ = Describe("MySQL Cluster E2E Tests", func() {
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
		testClusterRegistrationInOrchestrator(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed(), "failed to get cluster %s", cluster.Name)
	})

	It("scales up and fails over a cluster", func() {
		scaleToTwoNodes(f, cluster)

		// remove master pod
		By("removing master pod and waiting to become terminated")
		podName := framework.GetNameForResource("sts", cluster) + "-0"
		podKey := types.NamespacedName{Name: podName, Namespace: f.Namespace.Name}

		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), podName, meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		Eventually(func() bool {
			pod := &core.Pod{}
			f.Client.Get(context.TODO(), podKey, pod)
			return pod.ObjectMeta.DeletionTimestamp != nil && !pod.ObjectMeta.DeletionTimestamp.IsZero()
		}, f.Timeout, POLLING).Should(BeTrue(), fmt.Sprintf("Pod '%s' did not become terminated", f.GetPodHostname(cluster, 0)))

		// check failover done, this is a regression test
		By("checking cluster failover is done")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionTrue, f.Timeout)

		// after some time node 0 should be up and should be slave
		By("testing that old master is now designated as slave")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionReplicating, core.ConditionTrue, f.Timeout)

		// test cluster to be ready
		By("testing cluster after failover")
		testClusterReadiness(f, cluster)
		testClusterRegistrationInOrchestrator(f, cluster, ExpectDesignatedMaster(1))
	})

	It("fails over and successfully re-clones master from replica", func() {
		scaleToTwoNodes(f, cluster)

		// remove master pod
		By("removing master pod, its PVC and waiting to become terminated")
		podName := framework.GetNameForResource("sts", cluster) + "-0"
		pvcName := "data-" + podName
		podKey := types.NamespacedName{Name: podName, Namespace: f.Namespace.Name}

		// delete PVC from master pod and wait for it to be removed
		deletePVCSynchronously(f, pvcName, cluster.Namespace, f.Timeout)

		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), podName, meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		Eventually(func() bool {
			pod := &core.Pod{}
			f.Client.Get(context.TODO(), podKey, pod)
			return pod.ObjectMeta.DeletionTimestamp != nil && !pod.ObjectMeta.DeletionTimestamp.IsZero()
		}, f.Timeout, POLLING).Should(BeTrue(), fmt.Sprintf("Pod '%s' did not become terminated", f.GetPodHostname(cluster, 0)))

		// check failover done, this is a regression test
		By("checking cluster failover is done")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionTrue, f.Timeout)

		// after some time node 0 should be up and should be slave
		By("testing that old master is now designated as slave")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionReplicating, core.ConditionTrue, f.Timeout)

		// test cluster to be ready
		By("testing cluster after failover")
		testClusterReadiness(f, cluster)
		testClusterRegistrationInOrchestrator(f, cluster, ExpectDesignatedMaster(1))
	})

	It("scales down a cluster", func() {
		scaleToTwoNodes(f, cluster)

		// check PVCs
		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())
		Eventually(f.GetClusterPVCsFn(cluster)).Should(HaveLen(2))

		// scale down the cluster
		cluster.Spec.Replicas = &one
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		By("testing that cluster is ready after scale down")
		testClusterReadiness(f, cluster)

		By("checking that PVCs gets deleted")
		Eventually(f.GetClusterPVCsFn(cluster), f.Timeout, POLLING).Should(HaveLen(1))

		By("scaling cluster to zero")
		// refresh cluster to prevent a update conflict
		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// scale down the cluster to zero
		zero := int32(0)
		cluster.Spec.Replicas = &zero
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		By("testing that cluster is ready after scale down")
		testClusterReadiness(f, cluster)

		// it must not delete the PVC 0
		Consistently(f.GetClusterPVCsFn(cluster), f.Timeout, POLLING).Should(HaveLen(1))
	})

	It("removes nodes with slave io stopped", func() {
		scaleToTwoNodes(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// stop slave
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE;")

		// expect node to be removed from service and status to be updated
		By("testing pod-1 replicating condition is set to false")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionReplicating, core.ConditionFalse, f.Timeout)

		By("testing that pod-1 has healthy label set to 'no'")
		podName := framework.GetNameForResource("sts", cluster) + "-1"
		podKey := types.NamespacedName{Name: podName, Namespace: f.Namespace.Name}
		Eventually(func() *core.Pod {
			pod := &core.Pod{}
			f.Client.Get(context.TODO(), podKey, pod)
			return pod
		}, f.Timeout, POLLING).Should(haveLabelWithValue("healthy", "no"))
	})

	It("removes lagging nodes", func() {
		one := int64(1)
		cluster.Spec.MaxSlaveLatency = &one
		scaleToTwoNodes(f, cluster)

		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// set delayed replication
		f.ExecSQLOnNode(cluster, 1, "root", pw, "STOP SLAVE; CHANGE MASTER TO MASTER_DELAY = 100; START SLAVE;")

		// expect node to be removed from service and status to be updated
		By("testing pod-1 lagged condition is set to true")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionLagged, core.ConditionTrue, f.Timeout)

		By("testing that pod-1 has healthy label set to 'no'")
		podName := framework.GetNameForResource("sts", cluster) + "-1"
		podKey := types.NamespacedName{Name: podName, Namespace: f.Namespace.Name}
		Eventually(func() *core.Pod {
			pod := &core.Pod{}
			f.Client.Get(context.TODO(), podKey, pod)
			return pod
		}, f.Timeout, POLLING).Should(haveLabelWithValue("healthy", "no"))
	})

	It("properly sets up and doesn't failover a read only cluster", func() {
		cluster.Spec.Replicas = &two
		cluster.Spec.ReadOnly = true
		Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())

		// test cluster to be ready
		By("testing cluster is ready after scaling to two nodes and setting it read only")
		testClusterReadiness(f, cluster)
		testClusterRegistrationInOrchestrator(f, cluster, ExpectReadOnlyCluster())

		// get cluster
		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed())

		// expect cluster to be marked read only
		By("testing the cluster to be read only")
		f.ClusterEventuallyCondition(cluster, api.ClusterConditionReadOnly, core.ConditionTrue, f.Timeout)

		// expect node to be marked as lagged and removed from service
		By("testing cluster node 0 to be read only")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionReadOnly, core.ConditionTrue, f.Timeout)

		// TODO: fix this test
		// // node 1 should not be in healthy service because is marked as lagged (heartbeat can't write to master anymore)
		// By("test cluster endpoints after delayed slave")
		// testClusterEndpoints(f, cluster, []int{0}, []int{0})

		// remove master pod
		podName := framework.GetNameForResource("sts", cluster) + "-0"
		podKey := types.NamespacedName{Name: podName, Namespace: f.Namespace.Name}

		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), podName, meta.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %s", podName)

		Eventually(func() bool {
			pod := &core.Pod{}
			f.Client.Get(context.TODO(), podKey, pod)
			return pod.ObjectMeta.DeletionTimestamp != nil && !pod.ObjectMeta.DeletionTimestamp.IsZero()
		}, f.Timeout, POLLING).Should(BeTrue(), fmt.Sprintf("Pod '%s' did not become terminated", f.GetPodHostname(cluster, 0)))

		// check failover to not be started
		By("ensuring that failover is not started")
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
		f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionReadOnly, core.ConditionTrue, f.Timeout)
	})

})

func scaleToTwoNodes(f *framework.Framework, cluster *api.MysqlCluster) {
	// scale up the cluster
	By("scaling cluster up to two replicas")
	cluster.Spec.Replicas = &two
	Expect(f.Client.Update(context.TODO(), cluster)).To(Succeed())
	testClusterReadiness(f, cluster)
	testClusterRegistrationInOrchestrator(f, cluster)
	// test pod-0 is master and pod-1 is slave
	f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 0), api.NodeConditionMaster, core.ConditionTrue, f.Timeout)
	f.NodeEventuallyCondition(cluster, f.GetPodHostname(cluster, 1), api.NodeConditionMaster, core.ConditionFalse, f.Timeout)
}

func deletePVCSynchronously(f *framework.Framework, pvcName, namespace string, timeout time.Duration) {
	pvc := &core.PersistentVolumeClaim{}
	pvcKey := types.NamespacedName{Name: pvcName, Namespace: namespace}

	// first delete the PVC then remove the finalizer
	Expect(f.Client.Get(context.TODO(), pvcKey, pvc)).To(Succeed(), "failed to get pvc %s", pvcName)
	Expect(f.Client.Delete(context.TODO(), pvc)).To(Succeed(), "Failed to delete pvc %s", pvcName)
	Expect(f.Client.Get(context.TODO(), pvcKey, pvc)).To(Succeed(), "failed to get pvc %s", pvcName)
	pvc.Finalizers = nil
	Expect(f.Client.Update(context.TODO(), pvc)).To(Succeed(), "Failed to remove finalizers from pvc %s", pvcName)

	pvcNotFound := fmt.Sprintf("persistentvolumeclaims \"%s\" not found", pvc.Name)
	Eventually(func() error {
		return f.Client.Get(context.TODO(), pvcKey, pvc)
	}, timeout, POLLING).Should(MatchError(pvcNotFound), "PVC did not delete in time '%s'", pvc.Name)
}

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
	f.ClusterEventuallyCondition(cluster, api.ClusterConditionFailoverAck, core.ConditionFalse, f.Timeout)
}

type clusterOrchestratorRegistrationOptions struct {
	DesignatedMaster int
	ReadOnlyCluster  bool
}
type clusterOrchestratorRegistrationOption func(*clusterOrchestratorRegistrationOptions)

func ExpectDesignatedMaster(idx int) clusterOrchestratorRegistrationOption {
	return func(o *clusterOrchestratorRegistrationOptions) {
		o.DesignatedMaster = idx
	}
}

func ExpectReadOnlyCluster() clusterOrchestratorRegistrationOption {
	return func(o *clusterOrchestratorRegistrationOptions) {
		o.ReadOnlyCluster = true
	}
}

// tests if the cluster is in orchestrator and is properly configured
func testClusterRegistrationInOrchestrator(f *framework.Framework, cluster *api.MysqlCluster, opts ...clusterOrchestratorRegistrationOption) {
	o := clusterOrchestratorRegistrationOptions{}
	for _, optFn := range opts {
		optFn(&o)
	}

	replicas := int(*cluster.Spec.Replicas)
	consistOfNodes := make([]gomegatypes.GomegaMatcher, replicas, replicas)

	for i := 0; i < replicas; i++ {
		readOnly := o.ReadOnlyCluster || i != o.DesignatedMaster

		consistOfNodes[i] = MatchFields(IgnoreExtras, Fields{
			"Key": Equal(orc.InstanceKey{
				Hostname: f.GetPodHostname(cluster, i),
				Port:     3306,
			}),
			"GTIDMode":          Equal("ON"),
			"IsUpToDate":        Equal(true),
			"IsRecentlyChecked": Equal(true),
			"Binlog_format":     Equal("ROW"),
			"ReadOnly":          Equal(readOnly),
		})
	}

	// check orchestrator nodes to be equal.
	timeout := time.Duration(replicas) * f.Timeout
	Eventually(func() []orc.Instance {
		insts, err := f.OrcClient.Cluster(framework.OrcClusterName(cluster))
		if err != nil {
			f.Log.Error(err, "can't find nodes in orchestrator")
			return nil
		}

		return insts

	}, timeout, POLLING).Should(ConsistOf(consistOfNodes), "Cluster is not configured correctly in orchestrator.")
}

func haveLabelWithValue(label, value string) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Labels": HaveKeyWithValue(label, value),
		}),
	}))
}
