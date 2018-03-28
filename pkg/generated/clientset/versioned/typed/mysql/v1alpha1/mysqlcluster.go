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
package v1alpha1

import (
	v1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	scheme "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// MysqlClustersGetter has a method to return a MysqlClusterInterface.
// A group's client should implement this interface.
type MysqlClustersGetter interface {
	MysqlClusters(namespace string) MysqlClusterInterface
}

// MysqlClusterInterface has methods to work with MysqlCluster resources.
type MysqlClusterInterface interface {
	Create(*v1alpha1.MysqlCluster) (*v1alpha1.MysqlCluster, error)
	Update(*v1alpha1.MysqlCluster) (*v1alpha1.MysqlCluster, error)
	UpdateStatus(*v1alpha1.MysqlCluster) (*v1alpha1.MysqlCluster, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.MysqlCluster, error)
	List(opts v1.ListOptions) (*v1alpha1.MysqlClusterList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MysqlCluster, err error)
	MysqlClusterExpansion
}

// mysqlClusters implements MysqlClusterInterface
type mysqlClusters struct {
	client rest.Interface
	ns     string
}

// newMysqlClusters returns a MysqlClusters
func newMysqlClusters(c *MysqlV1alpha1Client, namespace string) *mysqlClusters {
	return &mysqlClusters{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the mysqlCluster, and returns the corresponding mysqlCluster object, and an error if there is any.
func (c *mysqlClusters) Get(name string, options v1.GetOptions) (result *v1alpha1.MysqlCluster, err error) {
	result = &v1alpha1.MysqlCluster{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("mysqlclusters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of MysqlClusters that match those selectors.
func (c *mysqlClusters) List(opts v1.ListOptions) (result *v1alpha1.MysqlClusterList, err error) {
	result = &v1alpha1.MysqlClusterList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("mysqlclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested mysqlClusters.
func (c *mysqlClusters) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("mysqlclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a mysqlCluster and creates it.  Returns the server's representation of the mysqlCluster, and an error, if there is any.
func (c *mysqlClusters) Create(mysqlCluster *v1alpha1.MysqlCluster) (result *v1alpha1.MysqlCluster, err error) {
	result = &v1alpha1.MysqlCluster{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("mysqlclusters").
		Body(mysqlCluster).
		Do().
		Into(result)
	return
}

// Update takes the representation of a mysqlCluster and updates it. Returns the server's representation of the mysqlCluster, and an error, if there is any.
func (c *mysqlClusters) Update(mysqlCluster *v1alpha1.MysqlCluster) (result *v1alpha1.MysqlCluster, err error) {
	result = &v1alpha1.MysqlCluster{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("mysqlclusters").
		Name(mysqlCluster.Name).
		Body(mysqlCluster).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *mysqlClusters) UpdateStatus(mysqlCluster *v1alpha1.MysqlCluster) (result *v1alpha1.MysqlCluster, err error) {
	result = &v1alpha1.MysqlCluster{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("mysqlclusters").
		Name(mysqlCluster.Name).
		SubResource("status").
		Body(mysqlCluster).
		Do().
		Into(result)
	return
}

// Delete takes name of the mysqlCluster and deletes it. Returns an error if one occurs.
func (c *mysqlClusters) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("mysqlclusters").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *mysqlClusters) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("mysqlclusters").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched mysqlCluster.
func (c *mysqlClusters) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MysqlCluster, err error) {
	result = &v1alpha1.MysqlCluster{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("mysqlclusters").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
