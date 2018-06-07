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

package clustercontroller

import (
	"sync"
	"time"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	"github.com/presslabs/mysql-operator/pkg/util/options"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

var _ = Describe("Test cluster reconciliation queue", func() {
	var (
		client     *fake.Clientset
		myClient   *fakeMyClient.Clientset
		rec        *record.FakeRecorder
		cluster    *api.MysqlCluster
		controller *Controller
		stop       chan struct{}
		opt        *options.Options
	)

	BeforeEach(func() {
		opt = options.GetOptions()
		client = fake.NewSimpleClientset()
		myClient = fakeMyClient.NewSimpleClientset()
		rec = record.NewFakeRecorder(100)
		cluster = tutil.NewFakeCluster("backupscheduler")
		stop = make(chan struct{})
		controller = newController(stop, client, myClient, rec)
		backupPollingTime = 10 * time.Millisecond
		backupWatchTimeout = time.Second

	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Scheduled backups cron", func() {
		Context("cluster with schedule backups", func() {
			It("try to register multiple times", func() {
				cluster.Spec.BackupSchedule = "0 * * * *"
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				err = controller.registerClusterInBackupCron(cluster)
				Expect(err).To(Succeed())

				Expect(controller.cron.Entries()).To(HaveLen(1))

				err = controller.registerClusterInBackupCron(cluster)
				Expect(err).To(Succeed())

				Expect(controller.cron.Entries()).To(HaveLen(1))
				Expect(controller.cron.Entries()).To(
					ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Job": MatchFields(IgnoreExtras, Fields{
							"Name":          Equal(cluster.Name),
							"Namespace":     Equal(cluster.Namespace),
							"BackupRunning": PointTo(Equal(false)),
						}),
					}))))
			})
			It("try to register multiple times in parallel", func() {
				cluster2 := tutil.NewFakeCluster("bs2")

				cluster.Spec.BackupSchedule = "0 * * * *"
				cluster2.Spec.BackupSchedule = "0 * * * *"

				go controller.registerClusterInBackupCron(cluster)
				go controller.registerClusterInBackupCron(cluster2)
				go controller.registerClusterInBackupCron(cluster)
				go controller.registerClusterInBackupCron(cluster2)

				Eventually(func() []*cron.Entry {
					lockJobRegister.Lock() // avoid a data race
					defer lockJobRegister.Unlock()
					return controller.cron.Entries()
				}).Should(HaveLen(2))
			})

			It("start job to schedule a backup", func() {
				j := job{
					Name:          cluster.Name,
					Namespace:     cluster.Namespace,
					myClient:      myClient,
					BackupRunning: new(bool),
					lock:          new(sync.Mutex),
				}
				go j.Run()
				go j.Run()

				Eventually(func() *bool {
					return j.BackupRunning
				}).Should(PointTo(Equal(true)))
				Eventually(func() []api.MysqlBackup {
					bs, _ := myClient.Mysql().MysqlBackups(j.Namespace).List(metav1.ListOptions{})
					return bs.Items
				}).Should(HaveLen(1))

				bs, _ := myClient.Mysql().MysqlBackups(j.Namespace).List(metav1.ListOptions{})
				backup := bs.Items[0]
				backup.Status.Completed = true
				_, err := myClient.Mysql().MysqlBackups(j.Namespace).Update(&backup)
				Expect(err).To(Succeed())

				Eventually(func() *bool {
					return j.BackupRunning
				}).Should(PointTo(Equal(false)))
			})
		})
	})
})
