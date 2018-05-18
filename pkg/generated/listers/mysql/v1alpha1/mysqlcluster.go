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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// MysqlClusterLister helps list MysqlClusters.
type MysqlClusterLister interface {
	// List lists all MysqlClusters in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.MysqlCluster, err error)
	// MysqlClusters returns an object that can list and get MysqlClusters.
	MysqlClusters(namespace string) MysqlClusterNamespaceLister
	MysqlClusterListerExpansion
}

// mysqlClusterLister implements the MysqlClusterLister interface.
type mysqlClusterLister struct {
	indexer cache.Indexer
}

// NewMysqlClusterLister returns a new MysqlClusterLister.
func NewMysqlClusterLister(indexer cache.Indexer) MysqlClusterLister {
	return &mysqlClusterLister{indexer: indexer}
}

// List lists all MysqlClusters in the indexer.
func (s *mysqlClusterLister) List(selector labels.Selector) (ret []*v1alpha1.MysqlCluster, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MysqlCluster))
	})
	return ret, err
}

// MysqlClusters returns an object that can list and get MysqlClusters.
func (s *mysqlClusterLister) MysqlClusters(namespace string) MysqlClusterNamespaceLister {
	return mysqlClusterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// MysqlClusterNamespaceLister helps list and get MysqlClusters.
type MysqlClusterNamespaceLister interface {
	// List lists all MysqlClusters in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.MysqlCluster, err error)
	// Get retrieves the MysqlCluster from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.MysqlCluster, error)
	MysqlClusterNamespaceListerExpansion
}

// mysqlClusterNamespaceLister implements the MysqlClusterNamespaceLister
// interface.
type mysqlClusterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all MysqlClusters in the indexer for a given namespace.
func (s mysqlClusterNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.MysqlCluster, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MysqlCluster))
	})
	return ret, err
}

// Get retrieves the MysqlCluster from the indexer for a given namespace and name.
func (s mysqlClusterNamespaceLister) Get(name string) (*v1alpha1.MysqlCluster, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("mysqlcluster"), name)
	}
	return obj.(*v1alpha1.MysqlCluster), nil
}
