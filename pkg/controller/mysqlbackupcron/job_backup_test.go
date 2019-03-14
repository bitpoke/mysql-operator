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

// nolint: errcheck
package mysqlbackupcron

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
)

const timeout = time.Second * 2

var _ = Describe("MysqlBackupCron cron job", func() {
	var (
		// controller k8s client
		c client.Client
		// stop channel for controller manager
		stop chan struct{}
	)

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).To(Succeed())
		c = mgr.GetClient()
		stop = StartTestManager(mgr)
	})
	AfterEach(func() {
		close(stop)
	})

	When("more backups are created", func() {
		var (
			clusterName string
			ns          string
			backups     []api.MysqlBackup
		)

		BeforeEach(func() {
			clusterName = fmt.Sprintf("cl-%d", rand.Int31())
			ns = "default"

			for i := 0; i < 10; i++ {
				backup := api.MysqlBackup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("bk-%d", i),
						Namespace: ns,
						Labels: map[string]string{
							"recurrent": "true",
						},
					},
					Spec: api.MysqlBackupSpec{
						ClusterName: clusterName,
					},
				}
				Expect(c.Create(context.TODO(), &backup)).To(Succeed())
				backups = append(backups, backup)
				time.Sleep(time.Second / 3)
			}
		})

		AfterEach(func() {
			for _, b := range backups {
				c.Delete(context.TODO(), &b)
			}
		})

		It("should delete only older backups", func() {
			limit := len(backups) - 5
			j := &job{
				ClusterName:                    clusterName,
				Namespace:                      ns,
				c:                              c,
				BackupScheduleJobsHistoryLimit: &limit,
			}
			lo := &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{
					"recurrent": "true",
				}),
				Namespace: ns,
			}
			Eventually(testutil.ListAllBackupsFn(c, lo)).Should(HaveLen(len(backups)))

			j.backupGC()

			Eventually(testutil.ListAllBackupsFn(c, lo)).Should(HaveLen(limit))
			Eventually(testutil.ListAllBackupsFn(c, lo)).ShouldNot(
				ContainElement(testutil.BackupWithName("bk-3")))
		})
	})
})
