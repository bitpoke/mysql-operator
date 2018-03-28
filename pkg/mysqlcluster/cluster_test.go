package mysqlcluster

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	fakeClientSet "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
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

func (f *cFactory) GetMysqlCluster() *api.MysqlCluster {
	return f.cluster
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
			SecretName: "the-db-secret",
		},
	}
}

func newFakeOption() *options.Options {
	opt := options.GetOptions()
	opt.Validate()
	return opt
}

var (
	opt *options.Options
)

func init() {
	opt = newFakeOption()
}

func getFakeFactory(name string) (*fake.Clientset, *fakeClientSet.Clientset,
	*record.FakeRecorder, *cFactory) {
	clientSet := fakeClientSet.NewSimpleClientset()
	clusterFake := newFakeCluster(name)
	if err := clusterFake.UpdateDefaults(opt); err != nil {
		panic(err)
	}

	k8sClient := fake.NewSimpleClientset()
	rec := record.NewFakeRecorder(100)

	return k8sClient, clientSet, rec, &cFactory{
		cluster:   clusterFake,
		client:    k8sClient,
		tiClient:  clientSet,
		namespace: DefaultNamespace,
		rec:       rec,
	}
}

func assertEqual(t *testing.T, left, right interface{}, msg string) {
	if !reflect.DeepEqual(left, right) {
		t.Errorf("%s ;Diff: %v == %v", msg, left, right)
	}
}

func TestSyncClusterCreation(t *testing.T) {
	_, _, _, f := getFakeFactory("test-cluster-create")

	ctx := context.TODO()
	if err := f.Sync(ctx); err != nil {
		t.Fail()
		return
	}
}
