package clustercontroller

import (
	"time"

	"github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	"github.com/presslabs/titanium/pkg/util/k8sutil"
)

var initRetryWaitTime = 30 * time.Second

type Controller struct {
	logger *logrus.Entry

	Namespace      string
	ServiceAccount string

	KubeCli    kubernetes.Interface
	KubeExtCli apiextensionsclient.Interface

	CreateCRD bool
}

func New(namespace string, serviceAccount string,
	kubecli kubernetes.Interface,
	kubeExtCli apiextensionsclient.Interface,
	createCRD bool,
) *Controller {
	return &Controller{
		logger:     logrus.WithField("pkg", "controller"),
		Namespace:  namespace,
		KubeCli:    kubecli,
		KubeExtCli: kubeExtCli,
		CreateCRD:  createCRD,

		//clusters: make(map[string]*cluster.Cluster),
	}
}

func (c *Controller) Start(workers int, stopCh <-chan struct{}) error {
	c.logger.Info("Starting controller ...")

	if !c.CreateCRD {
		err := c.createCRDIfNotExists()
		if err != nil {
			return err
		}
	}
	// create informar that watch for events.
	return nil
}

func (c *Controller) createCRDIfNotExists() error {
	c.logger.Info("Creating CRD...")

	err := k8sutil.CreateCRD(
		c.KubeExtCli,
		api.MysqlClusterCRDName,
		api.MysqlClusterCRDKind,
		api.MysqlClusterCRDPlural,
		"mysql",
	)
	if err != nil {
		c.logger.Error("Faild to create CRD: %v", err)
		return err
	}
	return k8sutil.WaitCRDReady(c.KubeExtCli, api.MysqlClusterCRDName)
}

const ControllerName = "mysqlclusterController"

func init() {
	controllerpkg.Register(ControllerName, func(ctx *controllerpkg.Context) controllerpkg.Interface {
		return New(
			ctx.Namespace,
			ctx.ServiceAccount,
			ctx.KubeCli,
			ctx.KubeExtCli,
			ctx.CreateCRD,
		).Start
	})
}
