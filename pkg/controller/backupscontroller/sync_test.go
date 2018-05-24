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

package backupscontroller

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiext_fake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

var _ = Describe("Test backup controller", func() {
	var (
		client     *fake.Clientset
		myClient   *fakeMyClient.Clientset
		crdClient  *apiext_fake.Clientset
		rec        *record.FakeRecorder
		cluster    *api.MysqlCluster
		backup     *api.MysqlBackup
		ctx        context.Context
		controller *Controller
		stop       chan struct{}
	)

	BeforeEach(func() {
		client = fake.NewSimpleClientset()
		myClient = fakeMyClient.NewSimpleClientset()
		crdClient = apiext_fake.NewSimpleClientset()
		rec = record.NewFakeRecorder(100)
		ctx = context.TODO()
		cluster = tutil.NewFakeCluster("asd")
		backup = tutil.NewFakeBackup("asd-backup", cluster.Name)
		stop = make(chan struct{})
		controller = newBackupController(stop, client, myClient, crdClient, rec)
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Controller syncing a backup", func() {
		Context("backup and cluster", func() {
			It("syncing a backup that is complet shuld be skiped", func() {
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				backup.Status.Completed = true

				err = controller.Sync(ctx, backup, tutil.Namespace)
				Expect(err).To(Succeed())
			})
			It("should fail because cluster name is not specified", func() {
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				backup.Spec.ClusterName = ""
				err = controller.Sync(ctx, backup, tutil.Namespace)
				Expect(err.Error()).To(ContainSubstring("cluster name is not specified"))
			})
		})
	})

})
