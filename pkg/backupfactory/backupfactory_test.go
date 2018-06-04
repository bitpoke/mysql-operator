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

package backupfactory

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"

	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

func TestBackupFactory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test backups factory")
}

func getFakeFactory(backup *api.MysqlBackup, k8Client *fake.Clientset,
	myClient *fakeMyClient.Clientset) *bFactory {

	cluster, err := myClient.MysqlV1alpha1().MysqlClusters(backup.Namespace).Get(
		backup.Spec.ClusterName, metav1.GetOptions{})

	Expect(err).To(Succeed())
	return &bFactory{
		backup:   backup,
		cluster:  cluster,
		k8Client: k8Client,
		myClient: myClient,
	}
}

var _ = Describe("Test backup factory", func() {
	var (
		client   *fake.Clientset
		myClient *fakeMyClient.Clientset

		ctx context.Context
	)

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		myClient = fakeMyClient.NewSimpleClientset()

		ctx = context.TODO()
	})

	Describe("Test a simple sync", func() {
		Context("A backup is created but not initialized", func() {
			It("sync should be successful", func() {
				cluster := tutil.NewFakeCluster("test-cluster-1")
				backup := tutil.NewFakeBackup("test-backup-1", cluster.Name)

				cluster.Spec.BackupUri = "gs://some-bucket/"
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				f := getFakeFactory(backup, client, myClient)
				Expect(f.SetDefaults()).To(Succeed())
				Expect(backup.Status.BackupUri).To(ContainSubstring("gs://some-bucket"))
				Expect(f.Sync(ctx)).To(Succeed())

				_, err = client.BatchV1().Jobs(tutil.Namespace).Get(
					f.getJobName(), metav1.GetOptions{})
				Expect(err).To(Succeed())
				Expect(f.Sync(ctx)).To(Succeed())
			})

			It("sync a backup with init bucket uri", func() {
				cluster := tutil.NewFakeCluster("test-cluster-1")
				backup := tutil.NewFakeBackup("test-backup-1", cluster.Name)

				backup.Spec.BackupUri = "gs://other-backup/test.gz"
				cluster.Spec.BackupUri = "gs://some-bucket/"
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				f := getFakeFactory(backup, client, myClient)
				Expect(f.SetDefaults()).To(Succeed())
				Expect(backup.Status.BackupUri).To(ContainSubstring("gs://other-backup"))
				Expect(f.Sync(ctx)).To(Succeed())
			})
			It("sync a backup with no bucket uri", func() {
				cluster := tutil.NewFakeCluster("test-cluster-1")
				backup := tutil.NewFakeBackup("test-backup-1", cluster.Name)

				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				f := getFakeFactory(backup, client, myClient)
				Expect(f.SetDefaults()).To(Succeed())
				Expect(f.Sync(ctx)).ToNot(Succeed())
			})
		})
	})
})
