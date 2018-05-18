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

// MysqlBackupLister helps list MysqlBackups.
type MysqlBackupLister interface {
	// List lists all MysqlBackups in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.MysqlBackup, err error)
	// MysqlBackups returns an object that can list and get MysqlBackups.
	MysqlBackups(namespace string) MysqlBackupNamespaceLister
	MysqlBackupListerExpansion
}

// mysqlBackupLister implements the MysqlBackupLister interface.
type mysqlBackupLister struct {
	indexer cache.Indexer
}

// NewMysqlBackupLister returns a new MysqlBackupLister.
func NewMysqlBackupLister(indexer cache.Indexer) MysqlBackupLister {
	return &mysqlBackupLister{indexer: indexer}
}

// List lists all MysqlBackups in the indexer.
func (s *mysqlBackupLister) List(selector labels.Selector) (ret []*v1alpha1.MysqlBackup, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MysqlBackup))
	})
	return ret, err
}

// MysqlBackups returns an object that can list and get MysqlBackups.
func (s *mysqlBackupLister) MysqlBackups(namespace string) MysqlBackupNamespaceLister {
	return mysqlBackupNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// MysqlBackupNamespaceLister helps list and get MysqlBackups.
type MysqlBackupNamespaceLister interface {
	// List lists all MysqlBackups in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.MysqlBackup, err error)
	// Get retrieves the MysqlBackup from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.MysqlBackup, error)
	MysqlBackupNamespaceListerExpansion
}

// mysqlBackupNamespaceLister implements the MysqlBackupNamespaceLister
// interface.
type mysqlBackupNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all MysqlBackups in the indexer for a given namespace.
func (s mysqlBackupNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.MysqlBackup, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MysqlBackup))
	})
	return ret, err
}

// Get retrieves the MysqlBackup from the indexer for a given namespace and name.
func (s mysqlBackupNamespaceLister) Get(name string) (*v1alpha1.MysqlBackup, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("mysqlbackup"), name)
	}
	return obj.(*v1alpha1.MysqlBackup), nil
}
