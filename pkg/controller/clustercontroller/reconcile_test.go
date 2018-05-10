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
	"context"
	"fmt"
	"testing"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
	"github.com/presslabs/mysql-operator/pkg/mysqlcluster"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

func newController(stop chan struct{}, client *fake.Clientset,
	myClient *fakeMyClient.Clientset,
	rec *record.FakeRecorder,
) *Controller {

	sharedInformerFactory := informers.NewSharedInformerFactory(
		myClient, time.Second)
	kubeSharedInformerFactory := kubeinformers.NewSharedInformerFactory(
		client, time.Second)

	sharedInformerFactory.Start(stop)
	kubeSharedInformerFactory.Start(stop)

	return New(
		client,
		myClient,
		kubeSharedInformerFactory,
		sharedInformerFactory,
		rec,
		tutil.Namespace,
	)
}

func TestReconcilation(t *testing.T) {
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()
	rec := record.NewFakeRecorder(100)

	stop := make(chan struct{})
	defer close(stop)
	controller := newController(stop, client, myClient, rec)

	cluster := tutil.NewFakeCluster("asd")
	_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
	if err != nil {
		fmt.Println("Failed to create cluster:", err)
	}

	ctx := context.TODO()
	err = controller.Reconcile(ctx, cluster)
	if err != nil {
		fmt.Println("Reconcile err: ", err)
		t.Fail()
	}

	_, shutdown := controller.reconcileQueue.Get()
	if shutdown {
		fmt.Println("shutdown")
		t.Fail()
	}
}
