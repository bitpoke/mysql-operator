package controller

import (
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	informers "github.com/presslabs/titanium/pkg/generated/informers/externalversions"
)

type Context struct {
	// KubeClient is a Kubernetes clientset
	KubeClient kubernetes.Interface
	// Client is a oxygen clientset
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
}
