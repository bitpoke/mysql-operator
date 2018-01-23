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
)

// The following function are helpers for accessing private members of cluster

func (c *cluster) SyncHeadlessService() error {
	return c.syncHeadlessService()
}

func (c *cluster) SyncConfigEnvSecret() error {
	return c.syncConfigEnvSecret()
}

func (c *cluster) SyncConfigMapFiles() error {
	return c.syncConfigMapFiles()
}

func (c *cluster) SyncStatefulSet() error {
	return c.syncStatefulSet()
}

func (c *cluster) GetMysqlCluster() *api.MysqlCluster {
	return c.cl
}

func (c *cluster) GetNameForResource(name ResourceName) string {
	return c.getNameForResource(name)
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
			Replicas:          2,
			MysqlRootPassword: "secure",
		},
	}
}

func getFakeCluster(name string) (*fake.Clientset, *fakeClientSet.Clientset, *cluster) {
	clientSet := fakeClientSet.NewSimpleClientset()
	clusterFake := newFakeCluster(name)
	if err := clusterFake.UpdateDefaults(); err != nil {
		panic(err)
	}

	k8sClient := fake.NewSimpleClientset()

	return k8sClient, clientSet, &cluster{
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
	client, _, c := getFakeCluster("test-cluster")

	tests := map[string]interface{}{
		"services":     func() error { return c.SyncHeadlessService() },
		"secrets":      func() error { return c.SyncConfigEnvSecret() },
		"configmaps":   func() error { return c.SyncConfigMapFiles() },
		"statefulsets": func() error { return c.SyncStatefulSet() },
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
	sfs, _ := sfC.Get(c.GetNameForResource(StatefulSet), metav1.GetOptions{})

	mc := c.GetMysqlCluster()
	assertEqual(t, *sfs.Spec.Replicas, mc.Spec.Replicas, "")

	assertEqual(t, sfs.Spec.Template.Spec.Containers[0].Image, mc.Spec.PodSpec.Image, "")
}

func TestSyncClusterIfExistsNoUpdate(t *testing.T) {
	client, _, c := getFakeCluster("test-cluster")
	ctx := context.TODO()
	c.Sync(ctx)
	client.ClearActions()

	tests := map[string]interface{}{
		"services":     func() error { return c.SyncHeadlessService() },
		"secrets":      func() error { return c.SyncConfigEnvSecret() },
		"configmaps":   func() error { return c.SyncConfigMapFiles() },
		"statefulsets": func() error { return c.SyncStatefulSet() },
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
	client, mcClient, c := getFakeCluster("test-cluster")
	ctx := context.TODO()
	c.Sync(ctx)
	mc := c.GetMysqlCluster()
	mc.Spec.Replicas = 4

	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)

	client.ClearActions()
	type tuple struct {
		f    func() error
		acts []string
	}
	tests := map[string]tuple{
		"services":   tuple{func() error { return c.SyncHeadlessService() }, []string{"get"}},
		"secrets":    tuple{func() error { return c.SyncConfigEnvSecret() }, []string{"get"}},
		"configmaps": tuple{func() error { return c.SyncConfigMapFiles() }, []string{"get"}},
		"statefulsets": tuple{
			func() error { return c.SyncStatefulSet() },
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
	sfs, _ := sfC.Get(c.GetNameForResource(StatefulSet), metav1.GetOptions{})

	assertEqual(t, *sfs.Spec.Replicas, mc.Spec.Replicas, "")
}

func TestPersistenceDisabledEnabled(t *testing.T) {
	client, mcClient, c := getFakeCluster("test-cluster-2")
	mc := c.GetMysqlCluster()

	assertEqual(t, mc.Spec.VolumeSpec.PersistenceEnabled, true, "persistenceEnabled default")

	mc.Spec.VolumeSpec.PersistenceEnabled = false
	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)
	c.SyncStatefulSet()

	sfC := client.AppsV1beta2().StatefulSets(DefaultNamespace)
	sfs, _ := sfC.Get(c.GetNameForResource(StatefulSet), metav1.GetOptions{})

	if sfs.Spec.Template.Spec.Volumes[2].EmptyDir == nil {
		t.Error("Is not empty dir.")
	}

	mc.Spec.VolumeSpec.PersistenceEnabled = true
	mcClient.Titanium().MysqlClusters(DefaultNamespace).Update(mc)
	c.SyncStatefulSet()

	sfs, _ = sfC.Get(c.GetNameForResource(StatefulSet), metav1.GetOptions{})
	if sfs.Spec.Template.Spec.Volumes[2].PersistentVolumeClaim == nil {
		t.Error("Is not a PVC.")
	}
	if sfs.Spec.Template.Spec.Volumes[2].EmptyDir != nil {
		t.Error("Is is an empty dir.")
	}
}
