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
	"fmt"
	"strings"
	"time"

	"database/sql"
	kutil_pf "github.com/appscode/kutil/tools/portforward"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8score "k8s.io/client-go/kubernetes/typed/core/v1"

	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var (
	TIMEOUT = 120 * time.Second
	POLLING = 2 * time.Second
)

func (f *Framework) ClusterEventuallyCondition(cluster *api.MysqlCluster,
	condType api.ClusterConditionType, status core.ConditionStatus) {
	Eventually(func() []api.ClusterCondition {
		c, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(f.Namespace.Name).Get(
			cluster.Name, meta.GetOptions{})
		if err != nil {
			return nil
		}
		return c.Status.Conditions
	}, TIMEOUT, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing cluster '%s' for condition %s to be %s", cluster.Name, condType, status)

}

func (f *Framework) NodeEventuallyCondition(cluster *api.MysqlCluster, nodeName string,
	condType api.NodeConditionType, status core.ConditionStatus) {
	Eventually(func() []api.NodeCondition {
		cluster, err := f.MyClientSet.MysqlV1alpha1().MysqlClusters(cluster.Namespace).Get(
			cluster.Name, meta.GetOptions{})
		if err != nil {
			return nil
		}

		for _, ns := range cluster.Status.Nodes {
			if ns.Name == nodeName {
				return ns.Conditions
			}
		}

		return nil
	}, TIMEOUT, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing node '%s' of the cluster '%s'", cluster.Name, nodeName)
}

func (f *Framework) ExecSQLOnNode(cluster *api.MysqlCluster, i int, user, password, query string) *sql.Rows {
	kubeCfg, err := LoadConfig()
	Expect(err).NotTo(HaveOccurred())

	podName := strings.Split(cluster.GetPodHostname(i), ".")[0]

	client := k8score.NewForConfigOrDie(kubeCfg).RESTClient()
	tunnel := kutil_pf.NewTunnel(client, kubeCfg, cluster.Namespace,
		podName,
		3306,
	)

	err = tunnel.ForwardPort()
	Expect(err).NotTo(HaveOccurred(), "Failed setting up port-forarding for pod: %s", podName)

	dsn := fmt.Sprintf("%s:%s@tcp(localhost:%d)/?timeout=10s&multiStatements=true", user, password, tunnel.Local)
	db, err := sql.Open("mysql", dsn)
	Expect(err).NotTo(HaveOccurred(), "Failed connection to mysql DSN: %s", dsn)

	rows, err := db.Query(query)
	Expect(err).NotTo(HaveOccurred(), "Query failed: %s", query)

	return rows
}

func (f *Framework) GetPodForNode(cluster *api.MysqlCluster, i int) *core.Pod {
	selector := labels.SelectorFromSet(cluster.GetLabels())
	podList, err := f.ClientSet.CoreV1().Pods(cluster.Namespace).List(meta.ListOptions{
		LabelSelector: selector.String(),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed listing pods for cluster '%s'", cluster.Name)

	hostname := cluster.GetPodHostname(i)
	for _, pod := range podList.Items {
		if strings.Contains(hostname, pod.Name) {
			return &pod
		}
	}

	return nil
}
