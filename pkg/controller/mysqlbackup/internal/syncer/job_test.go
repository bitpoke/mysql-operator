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

package syncer

import (
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var _ = Describe("MysqlBackup job syncer", func() {
	var (
		cluster *mysqlcluster.MysqlCluster
		backup  *mysqlbackup.MysqlBackup
		syncer  *jobSyncer
	)

	BeforeEach(func() {
		clusterName := fmt.Sprintf("cluster-%d", rand.Int31())
		name := fmt.Sprintf("backup-%d", rand.Int31())
		ns := "default"

		two := int32(2)
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   &two,
				SecretName: "a-secret",
			},
		})

		backup = mysqlbackup.New(&api.MysqlBackup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: api.MysqlBackupSpec{
				ClusterName:      clusterName,
				BackupURL:        "gs://bucket/",
				BackupSecretName: "secert",
			},
		})

		syncer = &jobSyncer{
			backup:  backup,
			cluster: cluster,
			opt:     options.GetOptions(),
		}
	})

	It("should return the master if nothing is status", func() {
		Expect(syncer.getBackupCandidate()).To(Equal(cluster.GetPodHostname(0)))
	})

	It("should return the healthy replica", func() {
		cluster.Status.Nodes = []api.NodeStatus{
			api.NodeStatus{
				Name:       cluster.GetPodHostname(0),
				Conditions: testutil.NodeConditions(true, false, false, false),
			},
			api.NodeStatus{
				Name:       cluster.GetPodHostname(1),
				Conditions: testutil.NodeConditions(false, true, false, true),
			},
		}
		Expect(syncer.getBackupCandidate()).To(Equal(cluster.GetPodHostname(1)))
	})

	It("should return the master if replicas is not healthy", func() {
		cluster.Status.Nodes = []api.NodeStatus{
			api.NodeStatus{
				Name:       cluster.GetPodHostname(0),
				Conditions: testutil.NodeConditions(true, false, false, false),
			},
			api.NodeStatus{
				Name:       cluster.GetPodHostname(1),
				Conditions: testutil.NodeConditions(false, false, false, true),
			},
		}
		Expect(syncer.getBackupCandidate()).To(Equal(cluster.GetPodHostname(0)))
	})
})
