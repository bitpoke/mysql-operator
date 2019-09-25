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
	batchv1 "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

const (
	POLLING = 2 * time.Second
)

var (
	one = int32(1)
	two = int32(2)
)

var _ = Describe("Mysql backups tests", func() {
	f := framework.NewFramework("mc-1")

	var (
		cluster      *api.MysqlCluster
		clusterKey   types.NamespacedName
		secret       *core.Secret
		backupSecret *core.Secret
		bucketName   string
		timeout      time.Duration
		rootPwd      string
		testData     string
	)

	BeforeEach(func() {
		// be careful, mysql allowed hostname lenght is <63
		name := fmt.Sprintf("cl-%d", rand.Int31()/1000)
		rootPwd = fmt.Sprintf("pw-%d", rand.Int31())
		bucketName = framework.GetBucketName()
		timeout = 350 * time.Second

		By("creating a new cluster secret")
		secret = framework.NewClusterSecret(name, f.Namespace.Name, rootPwd)
		Expect(f.Client.Create(context.TODO(), secret)).To(Succeed(), "create cluster secret")

		By("create a new backup secret")
		backupSecret = f.NewGCSBackupSecret()
		Expect(f.Client.Create(context.TODO(), backupSecret)).To(Succeed(), "create backup secret")

		By("creating a new cluster")
		cluster = framework.NewCluster(name, f.Namespace.Name)
		clusterKey = types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		cluster.Spec.BackupSecretName = backupSecret.Name
		Expect(f.Client.Create(context.TODO(), cluster)).To(Succeed(),
			"failed to create cluster '%s'", cluster.Name)

		By("testing the cluster readiness")
		Eventually(f.RefreshClusterFn(cluster), f.Timeout, POLLING).Should(
			framework.HaveClusterReplicas(1))
		Eventually(f.RefreshClusterFn(cluster), f.Timeout, POLLING).Should(
			framework.HaveClusterCond(api.ClusterConditionReady, core.ConditionTrue))

		// refresh cluster
		Expect(f.Client.Get(context.TODO(), clusterKey, cluster)).To(Succeed(),
			"failed to get cluster %s", cluster.Name)

		// write test data to the cluster
		testData = f.WriteSQLTest(cluster, 0, rootPwd)
	})

	Describe("tests with a good backup", func() {
		var (
			backup *api.MysqlBackup
		)
		BeforeEach(func() {
			By("create a backup for cluster")
			backup = framework.NewBackup(cluster, bucketName)
			Expect(f.Client.Create(context.TODO(), backup)).To(Succeed())
		})

		AfterEach(func() {
			// delete the backup that was created
			f.Client.Delete(context.TODO(), backup)
		})

		It("should be successful and can be restored", func() {
			// check that the data is in the cluster
			actual := f.ReadSQLTest(cluster, 0, rootPwd)
			Expect(actual).To(Equal(testData))

			By("check backup was successful")
			// should be backup successfully
			Eventually(f.RefreshBackupFn(backup), timeout, POLLING).Should(
				framework.BackupCompleted())
			Eventually(f.RefreshBackupFn(backup), timeout, POLLING).Should(
				framework.HaveBackupCond(api.BackupComplete, core.ConditionTrue))

			backup = f.RefreshBackupFn(backup)()

			By("create a new cluster from init bucket")
			// create cluster secret
			name := fmt.Sprintf("cl-%d-2", rand.Int31()/1000)
			sct := framework.NewClusterSecret(name, f.Namespace.Name, rootPwd)
			Expect(f.Client.Create(context.TODO(), sct)).To(Succeed(), "create cluster secret")

			// create cluster
			cl := framework.NewCluster(name, f.Namespace.Name)
			cl.Spec.InitBucketSecretName = backupSecret.Name
			cl.Spec.InitBucketURL = backup.Spec.BackupURL
			Expect(f.Client.Create(context.TODO(), cl)).To(Succeed(),
				"failed to create cluster '%s'", cluster.Name)

			// wait for cluster to be ready
			Eventually(f.RefreshClusterFn(cl), f.Timeout, POLLING).Should(
				framework.HaveClusterReplicas(1))
			Eventually(f.RefreshClusterFn(cl), f.Timeout, POLLING).Should(
				framework.HaveClusterCond(api.ClusterConditionReady, core.ConditionTrue))

			// check the data that was read before
			actual = f.ReadSQLTest(cl, 0, rootPwd)
			Expect(actual).To(Equal(testData))

		})
	})

	It("should fail the backup if bucket is not specified", func() {
		backup := framework.NewBackup(cluster, "gs://")
		Expect(f.Client.Create(context.TODO(), backup)).To(Succeed())

		localTimeout := 150 * time.Second
		// checks for the job because the backup is updated after the job is
		// marked as failed
		Eventually(func() *batchv1.Job {
			j := &batchv1.Job{}
			key := types.NamespacedName{
				Name:      framework.GetNameForJob(backup),
				Namespace: backup.Namespace,
			}
			f.Client.Get(context.TODO(), key, j)
			return j
		}, localTimeout, POLLING).Should(WithTransform(
			func(j *batchv1.Job) int32 { return j.Status.Failed }, BeNumerically(">", 2)))
	})
})
