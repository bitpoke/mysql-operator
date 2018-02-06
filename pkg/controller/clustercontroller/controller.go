package clustercontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	mcinformers "github.com/presslabs/titanium/pkg/generated/informers/externalversions/titanium/v1alpha1"
	mclisters "github.com/presslabs/titanium/pkg/generated/listers/titanium/v1alpha1"
	"github.com/presslabs/titanium/pkg/util"
	"github.com/presslabs/titanium/pkg/util/k8sutil"
	"github.com/presslabs/titanium/pkg/util/options"
)

const (
	initRetryWaitTime = 30 * time.Second
	workerPeriodTime  = 1 * time.Second

	ControllerName = "mysqlclusterController"
)

type Controller struct {
	Namespace      string
	ServiceAccount string

	KubeCli    kubernetes.Interface
	KubeExtCli apiextensionsclient.Interface

	CreateCRD bool

	clusterInformerSync cache.InformerSynced
	clusterLister       mclisters.MysqlClusterLister
	mcclient            clientset.Interface

	queue    workqueue.RateLimitingInterface
	workerWg sync.WaitGroup

	opt *options.Options
}

func New(mysqlClusterInformer mcinformers.MysqlClusterInformer,
	namespace string, serviceAccount string,
	kubecli kubernetes.Interface,
	kubeExtCli apiextensionsclient.Interface,
	createCRD bool,
	mcclient clientset.Interface,
	opt *options.Options,
) *Controller {
	ctrl := &Controller{
		Namespace:  namespace,
		KubeCli:    kubecli,
		KubeExtCli: kubeExtCli,
		CreateCRD:  createCRD,
		mcclient:   mcclient,
		opt:        opt,

		//clusters: make(map[string]*cluster.Cluster),
	}

	ctrl.queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mysqlcluster")

	// add handlers.
	mysqlClusterInformer.Informer().AddEventHandler(&controllerpkg.QueuingEventHandler{Queue: ctrl.queue})
	ctrl.clusterInformerSync = mysqlClusterInformer.Informer().HasSynced
	ctrl.clusterLister = mysqlClusterInformer.Lister()

	return ctrl

}

func (c *Controller) Start(workers int, stopCh <-chan struct{}) error {
	glog.V(2).Info("Starting controller ...")

	if !c.CreateCRD {
		err := c.createCRDIfNotExists()
		if err != nil {
			return err
		}
	}

	if !cache.WaitForCacheSync(stopCh, c.clusterInformerSync) {
		return fmt.Errorf("error waiting for informer cache to sync.")
	}

	for i := 0; i < workers; i++ {
		c.workerWg.Add(1)
		go wait.Until(func() { c.work(stopCh) }, workerPeriodTime, stopCh)
	}
	<-stopCh
	glog.V(2).Info("Shutting down controller.")
	c.queue.ShutDown()
	glog.V(2).Info("Wait for workers to exit...")
	c.workerWg.Wait()
	glog.V(2).Info("Workers exited.")
	return nil
}

func (c *Controller) work(stopCh <-chan struct{}) {
	defer c.workerWg.Done()
	glog.V(2).Info("Starting worker.")
	for {
		obj, shutdown := c.queue.Get()
		if shutdown {
			break
		}

		var key string
		err := func(obj interface{}) error {
			defer c.queue.Done(obj)

			var ok bool
			if key, ok = obj.(string); !ok {
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			ctx = util.ContextWithStopCh(ctx, stopCh)
			defer cancel() // TODO: is safe?

			glog.V(2).Info(fmt.Sprintf("[%s controller]: syncing item '%s'", ControllerName, key))

			// process items from queue
			if err := c.processNextWorkItem(ctx, key); err != nil {
				return err
			}

			c.queue.Forget(obj)
			return nil
		}(obj)

		if err != nil {
			glog.Errorf("%s controller: Re-queuing item %q due to error processing: %s",
				ControllerName, key, err.Error(),
			)
			c.queue.AddRateLimited(obj)
			continue
		}
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	mysqlCluster, err := c.clusterLister.MysqlClusters(namespace).Get(name)

	if err != nil {
		if k8sutil.IsKubernetesResourceNotFoundError(err) {
			runtime.HandleError(fmt.Errorf("issuer %q in work queue no longer exists", key))
			glog.Errorf("resource not found: %s", err)
			// TODO: fix deletion
			return nil
		}

		return err
	}

	return c.Sync(ctx, mysqlCluster, namespace)
}

func (c *Controller) createCRDIfNotExists() error {
	glog.V(2).Info("Creating CRD...")

	err := k8sutil.CreateCRD(
		c.KubeExtCli,
		api.MysqlClusterCRDName,
		api.MysqlClusterCRDKind,
		api.MysqlClusterCRDPlural,
		"mysql",
	)
	if err != nil {
		glog.Errorf("Faild to create CRD: %v", err)
		return err
	}
	return k8sutil.WaitCRDReady(c.KubeExtCli, api.MysqlClusterCRDName)
}

func init() {
	controllerpkg.Register(ControllerName, func(ctx *controllerpkg.Context) controllerpkg.Interface {
		return New(
			ctx.SharedInformerFactory.Titanium().V1alpha1().MysqlClusters(),
			ctx.Namespace,
			ctx.ServiceAccount,
			ctx.KubeCli,
			ctx.KubeExtCli,
			ctx.CreateCRD,
			ctx.MCClient,
			ctx.Opt,
		).Start
	})
}
