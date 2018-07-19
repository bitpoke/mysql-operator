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
	"testing"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestsyncPDB(t *testing.T) {
	ns := tutil.Namespace
	client := fake.NewSimpleClientset()
	myClient := fakeMyClient.NewSimpleClientset()

	// test 1 : minAvailable == nil && replicas <= 1 => delete PDB

	cluster := tutil.NewFakeCluster("asd")
	cluster.Spec.MinAvailable = nil
	cluster.Spec.Replicas = 1
	_, f := getFakeFactory(ns, cluster, client, myClient)
	state, err := f.syncPDB()
	if err != nil && state != statusDeleted {
		t.Errorf("PDB not deleted")
		t.Fail()
	}

	// test 2 : minAvailable == nil && replicas > 1 => minAvailable = DefaultMinAvailable

	cluster.Spec.MinAvailable = nil
	cluster.Spec.Replicas = 4
	_, f = getFakeFactory(ns, cluster, client, myClient)
	state, err = f.syncPDB()
	if err != nil && f.cluster.Spec.MinAvailable.IntValue() != api.DefaultMinAvailable.IntValue() {
		t.Errorf("PDB is not default")
		t.Fail()
	}

	// test 3 : minAvailable != nil

	myMinAvailable := intstr.FromString("80%")
	cluster.Spec.MinAvailable = &myMinAvailable
	_, f = getFakeFactory(ns, cluster, client, myClient)
	state, err = f.syncPDB()
	if err != nil && f.cluster.Spec.MinAvailable.IntValue() != myMinAvailable.IntValue() {
		t.Errorf("MinAvailable is incorect")
		t.Fail()
	}
}
