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
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
)

const (
	namespace = "default"
)

func init() {

	// make tests verbose
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "5")
}

func newFakeCluster(myClient *fakeMyClient.Clientset, name string) *api.MysqlCluster {
	cluster := &api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: api.ClusterSpec{
			Replicas:   1,
			SecretName: name,
		},
	}

	_, err := myClient.MysqlV1alpha1().MysqlClusters(namespace).Create(cluster)
	if err != nil {
		fmt.Println("Failed to create cluster:", err)
	}

	return cluster
}

func newFakeBackup(name, clName string) *api.MysqlBackup {
	return &api.MysqlBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.BackupSpec{
			ClusterName:      clName,
			BackupUri:        "gs://bucket/a.xb.gz",
			BackupSecretName: name,
		},
	}
}

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
		sharedInformerFactory.Mysql().V1alpha1().MysqlBackups(),
		sharedInformerFactory.Mysql().V1alpha1().MysqlClusters(),
		rec,
		namespace,
		kubeSharedInformerFactory.Batch().V1().Jobs(),
	)
}

// TestBackupCompleteSync
// Test: a backup already  completed
// Expect: skip sync-ing
func TestBackupCompleteSync(t *testing.T) {
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()
	rec := record.NewFakeRecorder(100)

	stop := make(chan struct{})
	defer close(stop)
	controller := newController(stop, client, myClient, rec)

	cluster := newFakeCluster(myClient, "asd")
	backup := newFakeBackup("asd-backup", cluster.Name)
	backup.Status.Completed = true

	ctx := context.TODO()
	err := controller.Sync(ctx, backup, namespace)
	if err != nil {
		fmt.Println("Sync err: ", err)
		t.Fail()
	}
}

// TestBackupSyncNoClusterName
// Test: backup without cluster name
// Expect: sync to fail
func TestBackupSyncNoClusterName(t *testing.T) {
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()
	rec := record.NewFakeRecorder(100)

	stop := make(chan struct{})
	defer close(stop)
	controller := newController(stop, client, myClient, rec)

	backup := newFakeBackup("asd-backup", "")
	backup.Status.Completed = true

	ctx := context.TODO()
	err := controller.Sync(ctx, backup, namespace)
	if !strings.Contains(err.Error(), "cluster name is not specified") {
		t.Fail()
	}
}
