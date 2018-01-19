package controller

import (
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"

	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	informers "github.com/presslabs/titanium/pkg/generated/informers/externalversions"
)

//  https://github.com/jetstack/cert-manager/blob/master/pkg/controller/context.go
type Context struct {
	Namespace      string
	ServiceAccount string

	KubeCli    kubernetes.Interface
	KubeExtCli apiextensionsclient.Interface

	CreateCRD bool

	MCClient clientset.Interface

	SharedInformerFactory informers.SharedInformerFactory
}
