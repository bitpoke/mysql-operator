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

package backupscontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	ticlientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	tiinformers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions/mysql/v1alpha1"
	mysqllisters "github.com/presslabs/mysql-operator/pkg/generated/listers/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util"
)

const (
	initRetryWaitTime = 30 * time.Second
	workerPeriodTime  = 1 * time.Second

	// ControllerName is the name of this controller
	ControllerName = "backupsController"
)

// Controller structure
type Controller struct {
	namespace string

	k8client kubernetes.Interface
	myClient ticlientset.Interface
	recorder record.EventRecorder

	jobLister     batchlisters.JobLister
	backupsLister mysqllisters.MysqlBackupLister
	clusterLister mysqllisters.MysqlClusterLister

	queue       workqueue.RateLimitingInterface
	workerWg    sync.WaitGroup
	syncedFuncs []cache.InformerSynced

	jobDeletionQueue workqueue.DelayingInterface
}

// New returns a new controller
func New(
	// kubernetes client
	k8client kubernetes.Interface,
	// clientset client
	myClient ticlientset.Interface,
	// mysql backups informer
	backupInformer tiinformers.MysqlBackupInformer,
	// mysql clusters informer
	clusterInformer tiinformers.MysqlClusterInformer,
	// event recorder
	eventRecorder record.EventRecorder,
	// the namespace
	namespace string,
	// job informer
	jobInformer batchinformers.JobInformer,

) *Controller {
	ctrl := &Controller{
		namespace: namespace,
		k8client:  k8client,
		myClient:  myClient,
		recorder:  eventRecorder,
	}
	// queues
	ctrl.queue = workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(), "mysqlbackup")
	ctrl.jobDeletionQueue = workqueue.NewNamedDelayingQueue("job-deletion-mysqlbackups")

	backupInformer.Informer().AddEventHandler(
		&controllerpkg.QueuingEventHandler{Queue: ctrl.queue})

	ctrl.backupsLister = backupInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, backupInformer.Informer().HasSynced)

	// job informer
	jobInformer.Informer().AddEventHandler(
		&controllerpkg.BlockingEventHandler{WorkFunc: ctrl.subresourceUpdated})
	ctrl.jobLister = jobInformer.Lister()
	ctrl.syncedFuncs = append(ctrl.syncedFuncs, jobInformer.Informer().HasSynced)

	ctrl.clusterLister = clusterInformer.Lister()
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

	// add delete job worker
	c.workerWg.Add(1)
	go wait.Until(func() { c.deleteJobWork(stopCh) }, workerPeriodTime, stopCh)

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
	glog.V(2).Info("Starting backup worker.")

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

	mysqlBackup, err := c.backupsLister.MysqlBackups(namespace).Get(name)

	if err != nil {
		if k8errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("backup %q in work queue no longer exists", key))
			glog.Errorf("resource not found: %s", err)
			return nil
		}

		return err
	}

	return c.Sync(ctx, mysqlBackup, namespace)
}

func (c *Controller) deleteJobWork(stopCh <-chan struct{}) {
	defer c.workerWg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = util.ContextWithStopCh(ctx, stopCh)
	defer cancel()

	glog.V(3).Info("Deletion job worker started.")

	for {
		obj, shutdown := c.jobDeletionQueue.Get()
		if shutdown {
			break
		}

		var key string
		err := func(obj interface{}) error {
			defer c.jobDeletionQueue.Done(obj)

			var ok bool
			if key, ok = obj.(string); !ok {
				return nil
			}

			// process items from queue
			if err := c.deleteJobWithKey(ctx, key); err != nil {
				return err
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

func (c *Controller) deleteJobWithKey(ctx context.Context, key string) error {
	glog.V(3).Infof("[deleteJobWithKey] deleting: %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	err = c.k8client.BatchV1().Jobs(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("job deletion: %s", err)
	}

	return nil
}

func init() {
	controllerpkg.Register(ControllerName, func(ctx *controllerpkg.Context) controllerpkg.Interface {
		return New(
			ctx.KubeClient,
			ctx.Client,
			ctx.SharedInformerFactory.Mysql().V1alpha1().MysqlBackups(),
			ctx.SharedInformerFactory.Mysql().V1alpha1().MysqlClusters(),
			ctx.Recorder,
			ctx.Namespace,
			ctx.KubeSharedInformerFactory.Batch().V1().Jobs(),
		).Start
	})
}
