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

package kube

import (
	"fmt"

	"github.com/appscode/kutil"
	"github.com/golang/glog"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiext_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func InstallCRD(crdClient apiext_clientset.Interface, crd *apiext.CustomResourceDefinition) error {
	glog.Info("Registering Custom Resource Definitions")

	existing, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.Name,
		metav1.GetOptions{})
	if k8errors.IsNotFound(err) {
		_, err = crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
		if err != nil {
			return fmt.Errorf("fail creating crd: %s", err)
		}
	} else if err != nil {
		return fmt.Errorf("fail getting crd: %s", err)
	} else {
		existing.Spec.Validation = crd.Spec.Validation
		if crd.Spec.Subresources != nil && existing.Spec.Subresources == nil {
			existing.Spec.Subresources = &apiext.CustomResourceSubresources{}
			if crd.Spec.Subresources.Status != nil && existing.Spec.Subresources.Status == nil {
				existing.Spec.Subresources.Status = crd.Spec.Subresources.Status
			}
			if crd.Spec.Subresources.Scale != nil && existing.Spec.Subresources.Scale == nil {
				existing.Spec.Subresources.Scale = crd.Spec.Subresources.Scale
			}
		}
		_, err = crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(existing)
		if err != nil {
			return fmt.Errorf("fail updating crd: %s", err)
		}
	}

	return nil
}

func WaitForCRD(crdClient apiext_clientset.Interface, crd *apiext.CustomResourceDefinition) error {
	return wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		_, err := crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
			crd.Name, metav1.GetOptions{})
		return err == nil, err
	})
}
