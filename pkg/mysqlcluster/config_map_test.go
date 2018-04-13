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
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

// TestConfigMapSync
// Test: create and update a config map
// Expect: object to exists and hash to be updated
func TestConfigMapSync(t *testing.T) {
	ns := tutil.Namespace
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	cluster := tutil.NewFakeCluster("asd")

	_, f := getFakeFactory(ns, cluster, client, myClient)

	action, err := f.syncConfigMysqlMap()
	if err != nil && action != "created" {
		t.Fail()
	}

	// ensure config map exists
	if _, err := client.CoreV1().ConfigMaps(ns).Get(
		cluster.GetNameForResource(api.ConfigMap),
		metav1.GetOptions{}); err != nil {
		t.Fail()
	}

	last_hash := f.configHash

	// patch does not work on fake client
	// https://github.com/kubernetes/client-go/issues/364
	// so check just hash to be different
	cluster.Spec.MysqlConf["ceva_nou"] = "1"
	action, err = f.syncConfigMysqlMap()
	if err != nil || action != "updated" {
		t.Fail()
	}

	if last_hash == f.configHash {
		t.Fail()
	}
}

// TestConfigMapData
// Test: get data for cluster
// Expect: data to be there
func TestConfigMapData(t *testing.T) {
	ns := tutil.Namespace
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	cluster := tutil.NewFakeCluster("asd")
	_, f := getFakeFactory(ns, cluster, client, myClient)

	// get first configs
	_, hash1, err := f.getMysqlConfData()
	if err != nil {
		fmt.Println("fail at first config")
		t.Fail()
	}

	// hash does not changed
	_, hash2, err := f.getMysqlConfData()
	if err != nil || hash1 != hash2 {
		fmt.Println("fail at second config")
		t.Fail()
	}

	// change configs
	cluster.Spec.MysqlConf["ceva_nou"] = "1"
	data, hash3, err := f.getMysqlConfData()
	if err != nil || hash1 == hash3 || !strings.Contains(data, "ceva_nou") {
		fmt.Printf("err: %s, last_hash: %s, hash: %s, data: %s\n", err, hash1, hash3, data)
		t.Fail()
	}
}
