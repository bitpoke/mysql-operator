/*
Copyright 2017 Google Inc.

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

package clientset

import (
	"fmt"

	dynamicdiscovery "github.com/appscode/kutil/dynamic/discovery"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type Clientset struct {
	config    rest.Config
	resources *dynamicdiscovery.ResourceMap
}

func New(config *rest.Config, resources *dynamicdiscovery.ResourceMap) *Clientset {
	return &Clientset{
		config:    *config,
		resources: resources,
	}
}

func (cs *Clientset) HasSynced() bool {
	return cs.resources.HasSynced()
}

func (cs *Clientset) Resource(apiVersion, resource, namespace string) (*ResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.Get(apiVersion, resource)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find resource %s in apiVersion %s", resource, apiVersion)
	}
	return cs.resource(apiResource, namespace)
}

func (cs *Clientset) Kind(apiVersion, kind, namespace string) (*ResourceClient, error) {
	// Look up the requested resource in discovery.
	apiResource := cs.resources.GetKind(apiVersion, kind)
	if apiResource == nil {
		return nil, fmt.Errorf("discovery: can't find kind %s in apiVersion %s", kind, apiVersion)
	}
	return cs.resource(apiResource, namespace)
}

func (cs *Clientset) resource(apiResource *dynamicdiscovery.APIResource, namespace string) (*ResourceClient, error) {
	// Create dynamic client for this apiVersion/resource.
	gv := apiResource.GroupVersion()
	config := cs.config
	config.GroupVersion = &gv
	if gv.Group != "" {
		config.APIPath = "/apis"
	}
	dc, err := dynamic.NewClient(&config)
	if err != nil {
		return nil, fmt.Errorf("can't create dynamic client for resource %v in apiVersion %v: %v", apiResource.Name, apiResource.APIVersion, err)
	}
	return &ResourceClient{
		ResourceInterface: dc.Resource(&apiResource.APIResource, namespace),
		dc:                dc,
		gv:                gv,
		resource:          apiResource,
	}, nil
}

type ResourceClient struct {
	dynamic.ResourceInterface

	dc       *dynamic.Client
	gv       schema.GroupVersion
	resource *dynamicdiscovery.APIResource
}

func (rc *ResourceClient) WithNamespace(namespace string) *ResourceClient {
	// Make a shallow copy of self, then change the namespace.
	rc2 := *rc
	rc2.ResourceInterface = rc.dc.Resource(&rc.resource.APIResource, namespace)
	return &rc2
}

func (rc *ResourceClient) Kind() string {
	return rc.resource.Kind
}

func (rc *ResourceClient) GroupVersion() schema.GroupVersion {
	return rc.gv
}

func (rc *ResourceClient) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    rc.gv.Group,
		Resource: rc.resource.Name,
	}
}

func (rc *ResourceClient) GroupVersionKind() schema.GroupVersionKind {
	return rc.gv.WithKind(rc.resource.Kind)
}

func (rc *ResourceClient) APIResource() *dynamicdiscovery.APIResource {
	return rc.resource
}

func (rc *ResourceClient) UpdateWithRetries(orig *unstructured.Unstructured, update func(obj *unstructured.Unstructured) bool) (result *unstructured.Unstructured, err error) {
	name := orig.GetName()
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		current, err := rc.Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.GetUID() != orig.GetUID() {
			// The original object was deleted and replaced with a new one.
			return apierrors.NewNotFound(rc.GroupResource(), name)
		}
		if changed := update(current); !changed {
			// There's nothing to do.
			result = current
			return nil
		}
		result, err = rc.Update(current)
		return err
	})
	return result, err
}
