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
package fake

import (
	v1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMysqlClusters implements MysqlClusterInterface
type FakeMysqlClusters struct {
	Fake *FakeMysqlV1alpha1
	ns   string
}

var mysqlclustersResource = schema.GroupVersionResource{Group: "mysql.presslabs.org", Version: "v1alpha1", Resource: "mysqlclusters"}

var mysqlclustersKind = schema.GroupVersionKind{Group: "mysql.presslabs.org", Version: "v1alpha1", Kind: "MysqlCluster"}

// Get takes name of the mysqlCluster, and returns the corresponding mysqlCluster object, and an error if there is any.
func (c *FakeMysqlClusters) Get(name string, options v1.GetOptions) (result *v1alpha1.MysqlCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(mysqlclustersResource, c.ns, name), &v1alpha1.MysqlCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlCluster), err
}

// List takes label and field selectors, and returns the list of MysqlClusters that match those selectors.
func (c *FakeMysqlClusters) List(opts v1.ListOptions) (result *v1alpha1.MysqlClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(mysqlclustersResource, mysqlclustersKind, c.ns, opts), &v1alpha1.MysqlClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MysqlClusterList{}
	for _, item := range obj.(*v1alpha1.MysqlClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested mysqlClusters.
func (c *FakeMysqlClusters) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(mysqlclustersResource, c.ns, opts))

}

// Create takes the representation of a mysqlCluster and creates it.  Returns the server's representation of the mysqlCluster, and an error, if there is any.
func (c *FakeMysqlClusters) Create(mysqlCluster *v1alpha1.MysqlCluster) (result *v1alpha1.MysqlCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(mysqlclustersResource, c.ns, mysqlCluster), &v1alpha1.MysqlCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlCluster), err
}

// Update takes the representation of a mysqlCluster and updates it. Returns the server's representation of the mysqlCluster, and an error, if there is any.
func (c *FakeMysqlClusters) Update(mysqlCluster *v1alpha1.MysqlCluster) (result *v1alpha1.MysqlCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(mysqlclustersResource, c.ns, mysqlCluster), &v1alpha1.MysqlCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMysqlClusters) UpdateStatus(mysqlCluster *v1alpha1.MysqlCluster) (*v1alpha1.MysqlCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(mysqlclustersResource, "status", c.ns, mysqlCluster), &v1alpha1.MysqlCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlCluster), err
}

// Delete takes name of the mysqlCluster and deletes it. Returns an error if one occurs.
func (c *FakeMysqlClusters) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(mysqlclustersResource, c.ns, name), &v1alpha1.MysqlCluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMysqlClusters) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(mysqlclustersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MysqlClusterList{})
	return err
}

// Patch applies the patch and returns the patched mysqlCluster.
func (c *FakeMysqlClusters) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MysqlCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mysqlclustersResource, c.ns, name, data, subresources...), &v1alpha1.MysqlCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlCluster), err
}
