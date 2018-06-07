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

package mysqlcluster

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

// The following function are helpers for accessing private members of cluster

func (f *cFactory) SyncHeadlessService() (string, error) {
	return f.syncHeadlessService()
}

func (f *cFactory) SyncConfigMapFiles() (string, error) {
	return f.syncConfigMysqlMap()
}

func (f *cFactory) SyncStatefulSet() (string, error) {
	return f.syncStatefulSet()
}

func (f *cFactory) GetComponents() []component {
	return f.getComponents()
}

func newFakeSecret(name, rootP string) *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string][]byte{
			"ROOT_PASSWORD": []byte(rootP),
		},
	}
}

func getFakeFactory(ns string, cluster *api.MysqlCluster, client *fake.Clientset,
	myClient *fakeMyClient.Clientset) (*record.FakeRecorder, *cFactory) {

	rec := record.NewFakeRecorder(100)
	opt := tutil.NewFakeOption()

	if err := cluster.UpdateDefaults(opt); err != nil {
		panic(err)
	}

	return rec, &cFactory{
		opt:       opt,
		cluster:   cluster,
		client:    client,
		myClient:  myClient,
		namespace: ns,
		rec:       rec,
	}
}

func assertEqual(t *testing.T, left, right interface{}, msg string) {
	if !reflect.DeepEqual(left, right) {
		t.Errorf("%s ;Diff: %v == %v", msg, left, right)
	}
}

// BEGIN TESTS

// TestSyncClusterCreationNoSecret
// Test: sync a cluster with a db secret name that does not exists.
// Expect: to fail cluster sync
func TestSyncClusterCreationNoSecret(t *testing.T) {
	ns := tutil.Namespace
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	cluster := tutil.NewFakeCluster("test-1")
	_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
	if err != nil {
		fmt.Println("Failed to create cluster:", err)
	}
	_, f := getFakeFactory(ns, cluster, client, myClient)

	ctx := context.TODO()
	err = f.Sync(ctx)

	if !strings.Contains(err.Error(), "secret 'test-1' failed") {
		t.Fail()
	}
}

// TestSyncClusterCreationWithSecret
// Test: sync a cluster with all required fields corectly
// Expect: sync successful, all elements created
func TestSyncClusterCreationWithSecret(t *testing.T) {
	ns := tutil.Namespace
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	sct := newFakeSecret("test-2", "Asd")
	client.CoreV1().Secrets(ns).Create(sct)

	cluster := tutil.NewFakeCluster("test-2")
	_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
	if err != nil {
		fmt.Println("Failed to create cluster:", err)
	}
	cluster.Spec.BackupSchedule = "* * * *"
	_, f := getFakeFactory(ns, cluster, client, myClient)

	ctx := context.TODO()
	if err := f.Sync(ctx); err != nil {
		t.Fail()
		return
	}

	fmt.Println(f.configHash)
	if f.configHash == "1" {
		t.Fail()
		return
	}

	_, err = client.CoreV1().ConfigMaps(ns).Get(cluster.GetNameForResource(api.ConfigMap), metav1.GetOptions{})
	if err != nil {
		t.Fail()
		return
	}

	_, err = client.CoreV1().Services(ns).Get(cluster.GetNameForResource(api.HeadlessSVC), metav1.GetOptions{})
	if err != nil {
		t.Fail()
		return
	}

	_, err = client.AppsV1().StatefulSets(ns).Get(cluster.GetNameForResource(api.StatefulSet), metav1.GetOptions{})
	if err != nil {
		t.Fail()
		return
	}
}
