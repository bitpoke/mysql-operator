package mysqlcluster

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	//k8sTest "k8s.io/client-go/testing"
	//clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	fakeClientSet "github.com/presslabs/titanium/pkg/generated/clientset/versioned/fake"
	"github.com/presslabs/titanium/pkg/util/options"
)

// The following function are helpers for accessing private members of cluster

func (f *cFactory) SyncHeadlessService() error {
	return f.syncHeadlessService()
}

func (f *cFactory) SyncConfigEnvSecret() error {
	return f.syncConfigEnvSecret()
}

func (f *cFactory) SyncConfigMapFiles() error {
	return f.syncConfigMapFiles()
}

func (f *cFactory) SyncStatefulSet() error {
	return f.syncStatefulSet()
}

func (f *cFactory) GetMysqlCluster() *api.MysqlCluster {
	return f.cl
}

func (f *cFactory) GetNameForResource(name ResourceName) string {
	return f.getNameForResource(name)
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
			ReadReplicas: 1,
			SecretName:   "the-db-secret",
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

func getFakeFactory(name string) (*fake.Clientset, *fakeClientSet.Clientset, *cFactory) {
	clientSet := fakeClientSet.NewSimpleClientset()
	clusterFake := newFakeCluster(name)
	if err := clusterFake.UpdateDefaults(opt); err != nil {
		panic(err)
	}

	k8sClient := fake.NewSimpleClientset()

	return k8sClient, clientSet, &cFactory{
		cl:        clusterFake,
		client:    k8sClient,
		cmclient:  clientSet,
		namespace: DefaultNamespace,
	}
}

func assertEqual(t *testing.T, left, right interface{}, msg string) {
	if !reflect.DeepEqual(left, right) {
		t.Errorf("%s ;Diff: %v == %v", msg, left, right)
	}
}

func TestSyncClusterCreation(t *testing.T) {
	client, _, f := getFakeFactory("test-cluster")

	tests := map[string]interface{}{
		"services":     func() error { return f.SyncHeadlessService() },
		"secrets":      func() error { return f.SyncConfigEnvSecret() },
		"configmaps":   func() error { return f.SyncConfigMapFiles() },
		"statefulsets": func() error { return f.SyncStatefulSet() },
	}

	for rName, syncF := range tests {
		// call the sync handler
		if err := syncF.(func() error)(); err != nil {
			t.Error("Can't sync cluster.")
		}
		acts := client.Actions()
		if len(acts) != 2 {
			t.Error("Different number of actions.")
			return
		}
		assertEqual(t, acts[0].GetVerb(), "get", "Not a get action.")
		assertEqual(t, acts[0].GetResource().Resource, rName, "Not the right resource.")
		assertEqual(t, acts[1].GetVerb(), "create", "Not a create action.")
		assertEqual(t, acts[1].GetResource().Resource, rName, "Not the right resource.")

		// clear actions
		client.ClearActions()
	}
	sfC := client.AppsV1beta2().StatefulSets(DefaultNamespace)
	sfs, _ := sfC.Get(f.GetNameForResource(StatefulSet), metav1.GetOptions{})

	mc := f.GetMysqlCluster()
	assertEqual(t, *sfs.Spec.Replicas, *mc.Spec.GetReplicas(), "")

	assertEqual(t, sfs.Spec.Template.Spec.Containers[0].Image, mc.Spec.GetMysqlImage(), "")
}

func TestSyncClusterIfExistsNoUpdate(t *testing.T) {
	client, _, f := getFakeFactory("test-cluster")
	ctx := context.TODO()
	f.Sync(ctx)
	client.ClearActions()

	tests := map[string]interface{}{
		"services":     func() error { return f.SyncHeadlessService() },
		"secrets":      func() error { return f.SyncConfigEnvSecret() },
		"configmaps":   func() error { return f.SyncConfigMapFiles() },
		"statefulsets": func() error { return f.SyncStatefulSet() },
	}

	for rName, syncF := range tests {
		// call the sync handler
		if err := syncF.(func() error)(); err != nil {
			t.Error("Can't sync cluster.")
		}

		acts := client.Actions()
		if len(acts) != 1 {
			t.Errorf("Different number of actions. Failed at: %s\n", rName)
			return
		}
		assertEqual(t, acts[0].GetVerb(), "get", "Not a get action.")
		assertEqual(t, acts[0].GetResource().Resource, rName, "Not the right resource.")

		// clear actions
		client.ClearActions()
	}
}

func TestSyncClusterIfExistsNeedsUpdate(t *testing.T) {
	client, mcClient, f := getFakeFactory("test-cluster-nu")
	ctx := context.TODO()
	f.Sync(ctx)
	mc := f.GetMysqlCluster()
	mc.Spec.ReadReplicas = 4
	mc.Spec.UpdateDefaults(opt)

	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)

	client.ClearActions()
	type tuple struct {
		f    func() error
		acts []string
	}
	tests := map[string]tuple{
		"services":   tuple{func() error { return f.SyncHeadlessService() }, []string{"get"}},
		"secrets":    tuple{func() error { return f.SyncConfigEnvSecret() }, []string{"get"}},
		"configmaps": tuple{func() error { return f.SyncConfigMapFiles() }, []string{"get"}},
		"statefulsets": tuple{
			func() error { return f.SyncStatefulSet() },
			[]string{"get", "update"},
		},
	}

	for rName, tup := range tests {
		// call the sync handler
		if err := tup.f(); err != nil {
			t.Error("Can't sync cluster.")
		}

		acts := client.Actions()
		if len(acts) != len(tup.acts) {
			t.Errorf("Different number of actions. Failed at: %s\n", rName)
			t.Errorf("Exp: %v, Actual: %v", tup.acts, acts)
			return
		}

		for i, aName := range tup.acts {
			assertEqual(t, acts[i].GetVerb(), aName, "Not a get action.")
		}
		assertEqual(t, acts[0].GetResource().Resource, rName, "Not the right resource.")

		// clear actions
		client.ClearActions()
	}

	sfC := client.AppsV1beta2().StatefulSets(DefaultNamespace)
	sfs, _ := sfC.Get(f.GetNameForResource(StatefulSet), metav1.GetOptions{})

	assertEqual(t, *sfs.Spec.Replicas, *mc.Spec.GetReplicas(), "")
}

func TestPersistenceDisabledEnabled(t *testing.T) {
	client, mcClient, f := getFakeFactory("test-cluster-2")
	mc := f.GetMysqlCluster()

	assertEqual(t, mc.Spec.VolumeSpec.PersistenceDisabled, false, "persistenceEnabled default")

	mc.Spec.VolumeSpec.PersistenceDisabled = true
	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)
	f.SyncStatefulSet()

	sfC := client.AppsV1beta2().StatefulSets(DefaultNamespace)
	sfs, _ := sfC.Get(f.GetNameForResource(StatefulSet), metav1.GetOptions{})

	if sfs.Spec.Template.Spec.Volumes[2].EmptyDir == nil {
		t.Error("Is not empty dir.")
	}
}

func TestPersistenceEnabled(t *testing.T) {
	client, mcClient, f := getFakeFactory("test-cluster-2")
	mc := f.GetMysqlCluster()
	mc.Spec.VolumeSpec.PersistenceDisabled = false

	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)
	f.SyncStatefulSet()

	sfC := client.AppsV1beta2().StatefulSets(DefaultNamespace)
	sfs, _ := sfC.Get(f.GetNameForResource(StatefulSet), metav1.GetOptions{})
	if sfs.Spec.Template.Spec.Volumes[2].PersistentVolumeClaim == nil {
		t.Error("Is not a PVC. And should be.")
	}
	if sfs.Spec.Template.Spec.Volumes[2].EmptyDir != nil {
		t.Error("Is is an empty dir. Should be a PVC")
	}
}
