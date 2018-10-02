/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lister

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type Lister struct {
	indexer       cache.Indexer
	groupResource schema.GroupResource
}

func New(groupResource schema.GroupResource, indexer cache.Indexer) *Lister {
	return &Lister{
		groupResource: groupResource,
		indexer:       indexer,
	}
}

func (l *Lister) List(selector labels.Selector) (ret []*unstructured.Unstructured, err error) {
	err = cache.ListAll(l.indexer, selector, func(obj interface{}) {
		ret = append(ret, obj.(*unstructured.Unstructured))
	})
	return ret, err
}

func (l *Lister) ListNamespace(namespace string, selector labels.Selector) (ret []*unstructured.Unstructured, err error) {
	err = cache.ListAllByNamespace(l.indexer, namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*unstructured.Unstructured))
	})
	return ret, err
}

func (l *Lister) Get(namespace, name string) (*unstructured.Unstructured, error) {
	key := name
	if namespace != "" {
		key = fmt.Sprintf("%s/%s", namespace, name)
	}
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(l.groupResource, name)
	}
	return obj.(*unstructured.Unstructured), nil
}
