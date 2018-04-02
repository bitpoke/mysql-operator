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
	"flag"
	"fmt"
	"testing"

	// core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
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

func getFakeFactory(backup *api.MysqlBackup, k8Client *fake.Clientset,
	myClient *fakeMyClient.Clientset) *bFactory {

	cluster, err := myClient.MysqlV1alpha1().MysqlClusters(backup.Namespace).Get(
		backup.Spec.ClusterName, metav1.GetOptions{})

	if err != nil {
		fmt.Println("Failed to get cluster:", err)
	}

	return &bFactory{
		backup:   backup,
		cluster:  cluster,
		k8Client: k8Client,
		myClient: myClient,
	}
}

// TestSync
// Test: sync a backup for a cluster
// Expect: sync successful, job created
func TestSync(t *testing.T) {
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	cluster := newFakeCluster(myClient, "test-2")
	backup := newFakeBackup("test-1", cluster.Name)
	f := getFakeFactory(backup, client, myClient)

	err := f.SetDefaults()
	if err != nil {
		t.Fail()
	}

	ctx := context.TODO()
	err = f.Sync(ctx)
	if err != nil {
		t.Fail()
	}

	_, err = client.BatchV1().Jobs(namespace).Get(f.getJobName(), metav1.GetOptions{})
	if err != nil {
		t.Fail()
		return
	}

	err = f.Sync(ctx)
	if err != nil {
		t.Fail()
	}
}
