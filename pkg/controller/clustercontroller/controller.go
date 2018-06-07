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

package clustercontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/wgliang/cron"
	apiext_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	myclientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	myinformers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
	mylisters "github.com/presslabs/mysql-operator/pkg/generated/listers/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
)

const (
	initRetryWaitTime = 30 * time.Second
	workerPeriodTime  = 1 * time.Second

	// ControllerName is the name of this controller
	ControllerName = "mysqlclusterController"

	reconcileWorkers = 4
)

// Controller structure
type Controller struct {
	namespace   string
	InstallCRDs bool

	k8client  kubernetes.Interface
	myClient  myclientset.Interface
	recorder  record.EventRecorder
	CRDClient apiext_clientset.Interface

	KubeSharedInformerFactory informers.SharedInformerFactory
	SharedInformerFactory     myinformers.SharedInformerFactory

	statefulSetLister appslisters.StatefulSetLister
	clusterLister     mylisters.MysqlClusterLister
	podLister         corelisters.PodLister

	queue       workqueue.RateLimitingInterface
	workerWg    sync.WaitGroup
	syncedFuncs []cache.InformerSynced

	clustersSync sync.Map

	reconcileQueue workqueue.DelayingInterface
	cron           *cron.Cron
}

// New returns a new controller
func New(ctx *controllerpkg.Context) *Controller {
	ctrl := &Controller{
		namespace:                 ctx.Namespace,
		k8client:                  ctx.KubeClient,
		myClient:                  ctx.Client,
		recorder:                  ctx.Recorder,
		InstallCRDs:               ctx.InstallCRDs,
		CRDClient:                 ctx.CRDClient,
		KubeSharedInformerFactory: ctx.KubeSharedInformerFactory,
		SharedInformerFactory:     ctx.SharedInformerFactory,
		cron: cron.New(),
	}

	statefulSetInformer := ctx.KubeSharedInformerFactory.Apps().V1().StatefulSets()
	podInformer := ctx.KubeSharedInformerFactory.Core().V1().Pods()
	mysqlClusterInformer := ctx.SharedInformerFactory.Mysql().V1alpha1().MysqlClusters()

	// MysqlCluster
	ctrl.queue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(), "mysqlcluster")

	ctrl.reconcileQueue = workqueue.NewNamedDelayingQueue("mysqlcluster-reconcile")

	mysqlClusterInformer.Informer().AddEventHandler(
		&controllerpkg.QueuingEventHandler{Queue: ctrl.queue})

	ctrl.clusterLister = mysqlClusterInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, mysqlClusterInformer.Informer().HasSynced)

	// StatefulSet
	statefulSetInformer.Informer().AddEventHandler(
		&controllerpkg.BlockingEventHandler{WorkFunc: ctrl.subresourceUpdated})

	ctrl.statefulSetLister = statefulSetInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, statefulSetInformer.Informer().HasSynced)

	ctrl.podLister = podInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, podInformer.Informer().HasSynced)

	return ctrl

}

// Start method start workers.
func (c *Controller) Start(workers int, stopCh <-chan struct{}) error {
	glog.Info("Starting controller ...")

	if c.InstallCRDs {
		glog.V(3).Info("Installing mysql crd...")
		if err := kube.InstallCRD(c.CRDClient, api.ResourceMysqlClusterCRD); err != nil {
			glog.Fatalf(err.Error())
			return fmt.Errorf("fail to create crd: %s", err)
		}
	}

	glog.V(3).Info("Wait for mysql crd...")
	if err := kube.WaitForCRD(c.CRDClient, api.ResourceMysqlClusterCRD); err != nil {
		return fmt.Errorf("crd does not exists: %s", err)
	}

	glog.V(4).Infof("Starting shared informer factory")
	c.SharedInformerFactory.Start(stopCh)
	c.KubeSharedInformerFactory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.syncedFuncs...) {
		return fmt.Errorf("error waiting for informer cache to sync")
	}

	// start cron
	c.cron.Start()

	for i := 0; i < workers; i++ {
		c.workerWg.Add(1)
		go wait.Until(func() { c.workerController(stopCh) }, workerPeriodTime, stopCh)
	}

	for i := 0; i < reconcileWorkers; i++ {
		c.workerWg.Add(1)
		go wait.Until(func() { c.workerRecouncile(stopCh) }, workerPeriodTime, stopCh)
	}

	<-stopCh
	glog.V(2).Info("Shutting down controller.")
	c.queue.ShutDown()
	c.reconcileQueue.ShutDown()
	c.cron.Stop()
	glog.V(2).Info("Wait for workers to exit...")
	c.workerWg.Wait()
	glog.V(2).Info("Workers exited.")
	return nil
}

func (c *Controller) workerController(stopCh <-chan struct{}) {
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
			defer cancel()

			glog.V(2).Info(fmt.Sprintf("[%s controller]: syncing item '%s'", ControllerName, key))

			// process items from queue
			cluster, err := c.getNextWorkItem(key)
			if err != nil && k8errors.IsNotFound(err) {
				// If the cluster has disappeared, do not re-queue
				glog.Infof("Removing issuer %q from processing queue", key)
				c.queue.Forget(obj)
				return nil
			}

			if err != nil {
				return fmt.Errorf("failed to get cluster: %s", err)
			}

			if err := c.Sync(ctx, cluster); err != nil {
				return fmt.Errorf("sync error: %s", err)
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

func (c *Controller) workerRecouncile(stopCh <-chan struct{}) {
	defer c.workerWg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = util.ContextWithStopCh(ctx, stopCh)
	defer cancel()

	glog.V(2).Info("Starting recouncile worker.")

	for {
		obj, shutdown := c.reconcileQueue.Get()
		if shutdown {
			break
		}

		var key string
		err := func(obj interface{}) error {
			defer c.reconcileQueue.Done(obj)

			var ok bool
			if key, ok = obj.(string); !ok {
				return nil
			}

			if ok1, ok2 := c.clustersSync.Load(key); !ok1.(bool) && ok2 {
				// key is removed from map, don't execute reconciliation
				return nil
			}

			// process items from queue
			cluster, err := c.getNextWorkItem(key)
			if err != nil {
				if k8errors.IsNotFound(err) {
					// key was removed from map, don't reconcile.
					glog.Infof("Removing issuer %q from reconcile queue", key)
					return nil
				}
				return fmt.Errorf("failed to get cluster: %s", err)
			}

			if err := c.Reconcile(ctx, cluster); err != nil {
				return fmt.Errorf("sync error: %s", err)
			}

			return nil
		}(obj)

		if err != nil {
			glog.Errorf("%s controller: Re-queuing item %q due to error processing: %s",
				ControllerName, key, err.Error(),
			)
			continue
		}
	}
}

func (c *Controller) getNextWorkItem(key string) (*api.MysqlCluster, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, fmt.Errorf("invalid resource key: %s", key)
	}

	if namespace != c.namespace {
		return nil, fmt.Errorf("received object with namespace '%s' that is not in working namespace '%s`",
			namespace, c.namespace)
	}

	mysqlCluster, err := c.clusterLister.MysqlClusters(namespace).Get(name)

	if err != nil {
		if k8errors.IsNotFound(err) {
			glog.Errorf("resource not found: %s", err)
			return nil, err
		}

		return nil, err
	}

	return mysqlCluster, nil
}

func (c *Controller) addClusterInWorkQueue(cluster *api.MysqlCluster) {
	key, err := controllerpkg.KeyFunc(cluster)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func init() {
	controllerpkg.Register(ControllerName, func(ctx *controllerpkg.Context) controllerpkg.Interface {
		return New(ctx).Start
	})
}
