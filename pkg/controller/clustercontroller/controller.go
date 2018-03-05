package clustercontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	mcinformers "github.com/presslabs/titanium/pkg/generated/informers/externalversions/titanium/v1alpha1"
	mclisters "github.com/presslabs/titanium/pkg/generated/listers/titanium/v1alpha1"
	"github.com/presslabs/titanium/pkg/util"
)

const (
	initRetryWaitTime = 30 * time.Second
	workerPeriodTime  = 1 * time.Second

	// ControllerName is the name of this controller
	ControllerName = "mysqlclusterController"
)

// Controller structure
type Controller struct {
	Namespace string

	KubeCli  kubernetes.Interface
	mcclient clientset.Interface
	recorder record.EventRecorder

	statefulSetLister appslisters.StatefulSetLister
	clusterLister     mclisters.MysqlClusterLister

	queue       workqueue.RateLimitingInterface
	workerWg    sync.WaitGroup
	syncedFuncs []cache.InformerSynced
}

// New returns a new controller
func New(
	// kubernetes client
	kubecli kubernetes.Interface,
	// clientset client
	mcclient clientset.Interface,
	// mysql cluster informer
	mysqlClusterInformer mcinformers.MysqlClusterInformer,
	// event recorder
	eventRecorder record.EventRecorder,
	// the namespace
	namespace string,
	// sfs informer
	statefulSetInformer appsinformers.StatefulSetInformer,

) *Controller {
	ctrl := &Controller{
		Namespace: namespace,
		KubeCli:   kubecli,
		mcclient:  mcclient,
		recorder:  eventRecorder,
	}

	// MysqlCluster
	ctrl.queue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(), "mysqlcluster")
	mysqlClusterInformer.Informer().AddEventHandler(
		&controllerpkg.QueuingEventHandler{Queue: ctrl.queue})
	ctrl.clusterLister = mysqlClusterInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, mysqlClusterInformer.Informer().HasSynced)

	// StatefulSet
	statefulSetInformer.Informer().AddEventHandler(
		&controllerpkg.BlockingEventHandler{WorkFunc: ctrl.subresourceUpdated})
	ctrl.statefulSetLister = statefulSetInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, statefulSetInformer.Informer().HasSynced)

	return ctrl

}

// Start method start workers.
func (c *Controller) Start(workers int, stopCh <-chan struct{}) error {
	glog.Info("Starting controller ...")

	if !cache.WaitForCacheSync(stopCh, c.syncedFuncs...) {
		return fmt.Errorf("error waiting for informer cache to sync")
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

	ctx, cancel := context.WithCancel(context.Background())
	ctx = util.ContextWithStopCh(ctx, stopCh)
	defer cancel() // TODO: is safe?

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
		if k8errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("issuer %q in work queue no longer exists", key))
			glog.Errorf("resource not found: %s", err)
			// TODO: fix deletion
			return nil
		}

		return err
	}

	return c.Sync(ctx, mysqlCluster, namespace)
}

func init() {
	controllerpkg.Register(ControllerName, func(ctx *controllerpkg.Context) controllerpkg.Interface {
		return New(
			ctx.KubeClient,
			ctx.Client,
			ctx.SharedInformerFactory.Titanium().V1alpha1().MysqlClusters(),
			ctx.Recorder,
			ctx.Namespace,
			ctx.KubeSharedInformerFactory.Apps().V1().StatefulSets(),
		).Start
	})
}
