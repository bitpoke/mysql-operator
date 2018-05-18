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

// FakeMysqlBackups implements MysqlBackupInterface
type FakeMysqlBackups struct {
	Fake *FakeMysqlV1alpha1
	ns   string
}

var mysqlbackupsResource = schema.GroupVersionResource{Group: "mysql.presslabs.org", Version: "v1alpha1", Resource: "mysqlbackups"}

var mysqlbackupsKind = schema.GroupVersionKind{Group: "mysql.presslabs.org", Version: "v1alpha1", Kind: "MysqlBackup"}

// Get takes name of the mysqlBackup, and returns the corresponding mysqlBackup object, and an error if there is any.
func (c *FakeMysqlBackups) Get(name string, options v1.GetOptions) (result *v1alpha1.MysqlBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(mysqlbackupsResource, c.ns, name), &v1alpha1.MysqlBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlBackup), err
}

// List takes label and field selectors, and returns the list of MysqlBackups that match those selectors.
func (c *FakeMysqlBackups) List(opts v1.ListOptions) (result *v1alpha1.MysqlBackupList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(mysqlbackupsResource, mysqlbackupsKind, c.ns, opts), &v1alpha1.MysqlBackupList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MysqlBackupList{}
	for _, item := range obj.(*v1alpha1.MysqlBackupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested mysqlBackups.
func (c *FakeMysqlBackups) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(mysqlbackupsResource, c.ns, opts))

}

// Create takes the representation of a mysqlBackup and creates it.  Returns the server's representation of the mysqlBackup, and an error, if there is any.
func (c *FakeMysqlBackups) Create(mysqlBackup *v1alpha1.MysqlBackup) (result *v1alpha1.MysqlBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(mysqlbackupsResource, c.ns, mysqlBackup), &v1alpha1.MysqlBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlBackup), err
}

// Update takes the representation of a mysqlBackup and updates it. Returns the server's representation of the mysqlBackup, and an error, if there is any.
func (c *FakeMysqlBackups) Update(mysqlBackup *v1alpha1.MysqlBackup) (result *v1alpha1.MysqlBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(mysqlbackupsResource, c.ns, mysqlBackup), &v1alpha1.MysqlBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlBackup), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMysqlBackups) UpdateStatus(mysqlBackup *v1alpha1.MysqlBackup) (*v1alpha1.MysqlBackup, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(mysqlbackupsResource, "status", c.ns, mysqlBackup), &v1alpha1.MysqlBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlBackup), err
}

// Delete takes name of the mysqlBackup and deletes it. Returns an error if one occurs.
func (c *FakeMysqlBackups) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(mysqlbackupsResource, c.ns, name), &v1alpha1.MysqlBackup{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMysqlBackups) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(mysqlbackupsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MysqlBackupList{})
	return err
}

// Patch applies the patch and returns the patched mysqlBackup.
func (c *FakeMysqlBackups) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MysqlBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mysqlbackupsResource, c.ns, name, data, subresources...), &v1alpha1.MysqlBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MysqlBackup), err
}
