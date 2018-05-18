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
	mysql_v1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	versioned "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/presslabs/mysql-operator/pkg/generated/listers/mysql/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	time "time"
)

// MysqlClusterInformer provides access to a shared informer and lister for
// MysqlClusters.
type MysqlClusterInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.MysqlClusterLister
}

type mysqlClusterInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewMysqlClusterInformer constructs a new informer for MysqlCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewMysqlClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredMysqlClusterInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredMysqlClusterInformer constructs a new informer for MysqlCluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredMysqlClusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MysqlV1alpha1().MysqlClusters(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MysqlV1alpha1().MysqlClusters(namespace).Watch(options)
			},
		},
		&mysql_v1alpha1.MysqlCluster{},
		resyncPeriod,
		indexers,
	)
}

func (f *mysqlClusterInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredMysqlClusterInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *mysqlClusterInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&mysql_v1alpha1.MysqlCluster{}, f.defaultInformer)
}

func (f *mysqlClusterInformer) Lister() v1alpha1.MysqlClusterLister {
	return v1alpha1.NewMysqlClusterLister(f.Informer().GetIndexer())
}
