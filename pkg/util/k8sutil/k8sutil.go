package k8sutil

import (
	"net"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // for gcp auth
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const ()

const TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

func MustNewKubeClient() kubernetes.Interface {
	cfg, err := ClusterConfig()
	if err != nil {
		panic(err)
	}
	return kubernetes.NewForConfigOrDie(cfg)
}

func InClusterConfig() (*rest.Config, error) {
	// Work around https://github.com/kubernetes/kubernetes/issues/40973
	// See https://github.com/coreos/etcd-operator/issues/731#issuecomment-283804819
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) == 0 {
		addrs, err := net.LookupHost("kubernetes.default.svc")
		if err != nil {
			panic(err)
		}
		os.Setenv("KUBERNETES_SERVICE_HOST", addrs[0])
	}
	if len(os.Getenv("KUBERNETES_SERVICE_PORT")) == 0 {
		os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	}
	return rest.InClusterConfig()
}

func ClusterConfig() (config *rest.Config, err error) {
	if len(os.Getenv("OUT_OF_KLUSTER")) != 0 {
		path := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", path)
		if err == nil {
			return
		}
	}

	config, err = InClusterConfig()
	return
}

func MustNewKubeExtClient() apiextensionsclient.Interface {
	cfg, err := ClusterConfig()
	if err != nil {
		panic(err)
	}
	return apiextensionsclient.NewForConfigOrDie(cfg)
}

func IsKubernetesResourceAlreadyExistError(err error) bool {
	exists := apierrors.IsAlreadyExists(err)
	if exists {
		logrus.Info("Resource already exists.")
	}
	return exists
}

func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}
