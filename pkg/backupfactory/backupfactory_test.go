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
	"fmt"
	"testing"

	// core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"

	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

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

	cluster := tutil.NewFakeCluster("test-2")
	_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
	if err != nil {
		fmt.Println("Failed to create cluster:", err)
	}
	backup := tutil.NewFakeBackup("test-1", cluster.Name)
	f := getFakeFactory(backup, client, myClient)

	err = f.SetDefaults()
	if err != nil {
		t.Fail()
	}

	ctx := context.TODO()
	err = f.Sync(ctx)
	if err != nil {
		t.Fail()
	}

	_, err = client.BatchV1().Jobs(tutil.Namespace).Get(f.getJobName(), metav1.GetOptions{})
	if err != nil {
		t.Fail()
		return
	}

	err = f.Sync(ctx)
	if err != nil {
		t.Fail()
	}
}
