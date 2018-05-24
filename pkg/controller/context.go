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

package controller

import (
	apiext_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	clientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
)

// Context contains the info that controller needs in order to run
type Context struct {
	// KubeClient is a Kubernetes clientset
	KubeClient kubernetes.Interface
	// Client is a mysql clientset
	Client clientset.Interface
	// Recorder to record events to
	Recorder record.EventRecorder

	// KubeSharedInformerFactory can be used to obtain shared
	// SharedIndexInformer instances for Kubernetes types
	KubeSharedInformerFactory kubeinformers.SharedInformerFactory
	// SharedInformerFactory can be used to obtain shared SharedIndexInformer
	// instances
	SharedInformerFactory informers.SharedInformerFactory

	// Namespace is a namespace to operate within. This should be used when
	// constructing SharedIndexInformers for the informer factory.
	Namespace string

	// InstallCRDs specifies if to install or not CRDs
	InstallCRDs bool

	// CRDClient is the clientset for Custom Resource Definitions
	CRDClient apiext_clientset.Interface
}
