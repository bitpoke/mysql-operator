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
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiext_fake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
)

func TestBackupController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test backups controller")
}

var _ = Describe("Test backup controller", func() {
	var (
		client    *fake.Clientset
		myClient  *fakeMyClient.Clientset
		crdClient *apiext_fake.Clientset

		rec        *record.FakeRecorder
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
		stop = make(chan struct{})
		controller = newBackupController(stop, client, myClient, crdClient, rec)
		controller.syncedFuncs = []cache.InformerSynced{func() bool { return true }}
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Test controller functionality", func() {
		Context("At controller startup, crds are not installed", func() {
			It("crd backup should be installed", func() {
				go controller.Start(0, stop)
				Eventually(func() error {
					_, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
						api.ResourceMysqlBackupCRDName, metav1.GetOptions{})
					return err
				}).Should(Succeed())
			})
			It("crd should not be installed", func() {
				controller.InstallCRDs = false
				go controller.Start(0, stop)
				Eventually(func() error {
					_, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
						api.ResourceMysqlBackupCRDName, metav1.GetOptions{})
					return err
				}).ShouldNot(Succeed())

			})
		})
	})

})

func newBackupController(stop chan struct{}, client *fake.Clientset,
	myClient *fakeMyClient.Clientset, crdClient *apiext_fake.Clientset,
	rec *record.FakeRecorder,
) *Controller {

	sharedInformerFactory := informers.NewSharedInformerFactory(
		myClient, time.Second)
	kubeSharedInformerFactory := kubeinformers.NewSharedInformerFactory(
		client, time.Second)

	sharedInformerFactory.Start(stop)
	kubeSharedInformerFactory.Start(stop)

	return New(&controllerpkg.Context{
		KubeClient: client,
		Client:     myClient,
		KubeSharedInformerFactory: kubeSharedInformerFactory,
		SharedInformerFactory:     sharedInformerFactory,
		Recorder:                  rec,
		InstallCRDs:               true,
		CRDClient:                 crdClient,
	})
}
