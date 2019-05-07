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

package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	"database/sql"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	k8score "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	pf "github.com/presslabs/mysql-operator/test/e2e/framework/portforward"
)

var (
	POLLING = 2 * time.Second
)

func (f *Framework) ClusterEventuallyCondition(cluster *api.MysqlCluster,
	condType api.ClusterConditionType, status corev1.ConditionStatus, timeout time.Duration) {
	Eventually(func() []api.ClusterCondition {
		key := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		if err := f.Client.Get(context.TODO(), key, cluster); err != nil {
			return nil
		}
		return cluster.Status.Conditions
	}, timeout, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing cluster '%s' for condition %s to be %s", cluster.Name, condType, status)

}

func (f *Framework) NodeEventuallyCondition(cluster *api.MysqlCluster, nodeName string,
	condType api.NodeConditionType, status corev1.ConditionStatus, timeout time.Duration) {
	Eventually(func() []api.NodeCondition {
		key := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		if err := f.Client.Get(context.TODO(), key, cluster); err != nil {
			return nil
		}

		for _, ns := range cluster.Status.Nodes {
			if ns.Name == nodeName {
				return ns.Conditions
			}
		}

		return nil
	}, timeout, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing node '%s' of the cluster '%s'", cluster.Name, nodeName)
}

func (f *Framework) ExecSQLOnNode(cluster *api.MysqlCluster, i int, user, password, query string) *sql.Rows {
	kubeCfg, err := LoadConfig()
	Expect(err).NotTo(HaveOccurred())

	podName := strings.Split(f.GetPodHostname(cluster, i), ".")[0]

	client := k8score.NewForConfigOrDie(kubeCfg).RESTClient()
	tunnel := pf.NewTunnel(client, kubeCfg, cluster.Namespace,
		podName,
		3306,
	)

	err = tunnel.ForwardPort()
	Expect(err).NotTo(HaveOccurred(), "Failed setting up port-forarding for pod: %s", podName)

	dsn := fmt.Sprintf("%s:%s@tcp(localhost:%d)/?timeout=10s&multiStatements=true", user, password, tunnel.Local)
	db, err := sql.Open("mysql", dsn)
	Expect(err).To(Succeed(), "Failed connection to mysql DSN: %s", dsn)

	rows, err := db.Query(query)
	Expect(err).To(Succeed(), "Query failed: %s", query)

	tunnel.Close()
	return rows
}

func (f *Framework) GetPodForNode(cluster *api.MysqlCluster, i int) *corev1.Pod {
	selector := labels.SelectorFromSet(cluster.GetLabels())
	podList, err := f.ClientSet.CoreV1().Pods(cluster.Namespace).List(metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed listing pods for cluster '%s'", cluster.Name)

	hostname := f.GetPodHostname(cluster, i)
	for _, pod := range podList.Items {
		if strings.Contains(hostname, pod.Name) {
			return &pod
		}
	}

	return nil
}

// GetPodHostname returns for an index the pod hostname of a cluster
func (f *Framework) GetPodHostname(cluster *api.MysqlCluster, p int) string {
	return fmt.Sprintf("%s-%d.%s.%s", GetNameForResource("sts", cluster), p,
		GetNameForResource("svc-headless", cluster),
		cluster.Namespace)
}

// GetNameForResource returns the name of the cluster resource, see the function
// definition for what name means.
func GetNameForResource(name string, cluster *api.MysqlCluster) string {
	switch name {
	case "sts":
		return fmt.Sprintf("%s-mysql", cluster.Name)
	case "svc-master":
		return fmt.Sprintf("%s-mysql-master", cluster.Name)
	case "svc-read":
		return fmt.Sprintf("%s-mysql", cluster.Name)
	case "svc-headless":
		return "mysql"
	default:
		return fmt.Sprintf("%s-mysql", cluster.Name)
	}
}

// HaveClusterCond is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
func HaveClusterCond(condType api.ClusterConditionType, status corev1.ConditionStatus) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(condType),
				"Status": Equal(status),
			})),
		})},
	))
}

func (f *Framework) RefreshClusterFn(cluster *api.MysqlCluster) func() *api.MysqlCluster {
	return func() *api.MysqlCluster {
		key := types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}
		c := &api.MysqlCluster{}
		f.Client.Get(context.TODO(), key, c)
		return c
	}
}

// HaveClusterRepliacs matcher for replicas
func HaveClusterReplicas(replicas int) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"ReadyNodes": Equal(replicas),
		}),
	}))
}

var (
	testDBName    = "op_test_w"
	testTableName = "op_table"
)

func (f *Framework) WriteSQLTest(cluster *api.MysqlCluster, pod int, pw string) string {
	By("run write SQL test to cluster")

	// create database
	f.ExecSQLOnNode(cluster, pod, "root", pw,
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", testDBName),
	)

	// create table
	f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`USE %s; CREATE TABLE IF NOT EXISTS %s
           (k varchar(20) NOT NULL, v varchar(36) NOT NULL, PRIMARY KEY (k));`,
		testDBName, testTableName,
	))

	// insert data
	data := string(uuid.NewUUID())
	f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`USE %s; INSERT INTO %s (k, v) VALUES ("data", "%s")
                    ON DUPLICATE KEY UPDATE k="data", v="%[3]s";`,
		testDBName, testTableName, data,
	))

	return data
}

func (f *Framework) ReadSQLTest(cluster *api.MysqlCluster, pod int, pw string) string {
	By("run read SQL test")
	var data string

	rows := f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`SELECT v FROM %s.%s WHERE k="data"`,
		testDBName, testTableName,
	))
	defer rows.Close()

	if rows.Next() {
		rows.Scan(&data)
	}

	return data
}

// GetClusterLabels returns labels.Set for the given cluster
func GetClusterLabels(cluster *api.MysqlCluster) labels.Set {
	labels := labels.Set{
		"mysql.presslabs.org/cluster": cluster.Name,
		"app.kubernetes.io/name":      "mysql",
	}

	return labels
}

func (f *Framework) GetClusterPVCsFn(cluster *api.MysqlCluster) func() []corev1.PersistentVolumeClaim {
	return func() []corev1.PersistentVolumeClaim {
		pvcList := &corev1.PersistentVolumeClaimList{}
		lo := &client.ListOptions{
			Namespace:     cluster.Namespace,
			LabelSelector: labels.SelectorFromSet(GetClusterLabels(cluster)),
		}
		f.Client.List(context.TODO(), lo, pvcList)
		return pvcList.Items
	}
}
