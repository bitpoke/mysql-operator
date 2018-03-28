package mysqlcluster

import (
	"context"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"testing"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeTiClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	"github.com/presslabs/mysql-operator/pkg/util/options"
)

// The following function are helpers for accessing private members of cluster

func (f *cFactory) SyncHeadlessService() (string, error) {
	return f.syncHeadlessService()
}

func (f *cFactory) SyncEnvSecret() (string, error) {
	return f.syncEnvSecret()
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

const (
	DefaultNamespace = "default"
)

func newFakeCluster(name string) *api.MysqlCluster {
	return &api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: api.ClusterSpec{
			Replicas:   1,
			SecretName: name,
		},
	}
}

func newFakeOption() *options.Options {
	opt := options.GetOptions()
	opt.Validate()
	return opt
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

var (
	opt *options.Options
)

func init() {
	opt = newFakeOption()

	// make tests verbose
	flag.Set("alsologtostderr", "true")
	flag.Set("v", "5")
}

func getFakeFactory(ns string, cluster *api.MysqlCluster, client *fake.Clientset,
	tiClient *fakeTiClient.Clientset) (*record.FakeRecorder, *cFactory) {
	if err := cluster.UpdateDefaults(opt); err != nil {
		panic(err)
	}

	rec := record.NewFakeRecorder(100)

	return rec, &cFactory{
		cluster:   cluster,
		client:    client,
		tiClient:  tiClient,
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
	ns := DefaultNamespace
	client := fake.NewSimpleClientset()
	tiClient := fakeTiClient.NewSimpleClientset()

	cluster := newFakeCluster("test-1")
	_, f := getFakeFactory(ns, cluster, client, tiClient)

	ctx := context.TODO()
	err := f.Sync(ctx)

	if !strings.Contains(err.Error(), "secret 'test-1' failed") {
		t.Fail()
	}
}

// TestSyncClusterCreationWithSecret
// Test: sync a cluster with all required fields corectly
// Expect: sync successful, all elements created
func TestSyncClusterCreationWithSecret(t *testing.T) {
	ns := DefaultNamespace
	client := fake.NewSimpleClientset()
	tiClient := fakeTiClient.NewSimpleClientset()

	sct := newFakeSecret("test-2", "Asd")
	client.CoreV1().Secrets(ns).Create(sct)

	cluster := newFakeCluster("test-2")
	_, f := getFakeFactory(ns, cluster, client, tiClient)

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

	var err error
	_, err = client.CoreV1().Secrets(ns).Get(cluster.GetNameForResource(api.EnvSecret), metav1.GetOptions{})
	if err != nil {
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
