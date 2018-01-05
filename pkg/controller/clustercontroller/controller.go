package clustercontroller

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	mcinformers "github.com/presslabs/titanium/pkg/generated/informers/externalversions/titanium/v1alpha1"
	mclisters "github.com/presslabs/titanium/pkg/generated/listers/titanium/v1alpha1"
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

	clusterLister mclisters.MysqlClusterLister

	queue    workqueue.RateLimitingInterface
	workerWg sync.WaitGroup
}

func New(mysqlClusterInformer mcinformers.MysqlClusterInformer,
	namespace string, serviceAccount string,
	kubecli kubernetes.Interface,
	kubeExtCli apiextensionsclient.Interface,
	createCRD bool,
) *Controller {
	ctrl := &Controller{
		logger:     logrus.WithField("pkg", "controller"),
		Namespace:  namespace,
		KubeCli:    kubecli,
		KubeExtCli: kubeExtCli,
		CreateCRD:  createCRD,

		//clusters: make(map[string]*cluster.Cluster),
	}

	ctrl.queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mysqlcluster")
	mysqlClusterInformer.Informer().AddEventHandler(&controllerpkg.QueuingEventHandler{Queue: ctrl.queue})

	ctrl.clusterLister = mysqlClusterInformer.Lister()

	return ctrl

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
			ctx.SharedInformerFactory.Titanium().V1alpha1().MysqlClusters(),
			ctx.Namespace,
			ctx.ServiceAccount,
			ctx.KubeCli,
			ctx.KubeExtCli,
			ctx.CreateCRD,
		).Start
	})
}
