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

package orchestratormysql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/options"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
	"github.com/presslabs/mysql-operator/pkg/util/stop"
)

const (
	eventNormal = "Normal"
	//eventWarning   = "Warning"
	controllerName = "orchestrator-mysql-controller"
)

var log = logf.Log.WithName(controllerName)

// reconcileTimePeriod represents the time in which a cluster shoud be reconciled
var reconcileTimePeriod = time.Second * 5

type eventLockMap struct {
	Map sync.Map
}

// newEventLockMap returns a map of clusters to events with custom methods to
// register an event or to remove an event
func newEventLockMap() eventLockMap {
	return eventLockMap{
		Map: sync.Map{},
	}
}

var clustersMap = newEventLockMap()

func (m *eventLockMap) getKey(meta metav1.Object) string {
	return fmt.Sprintf("%s/%s", meta.GetNamespace(), meta.GetName())
}

func (m *eventLockMap) CreateEvent(evt event.CreateEvent) {
	m.Map.Store(m.getKey(evt.Meta), event.GenericEvent{ // nolint: megacheck
		Meta:   evt.Meta,
		Object: evt.Object,
	})
}

func (m *eventLockMap) DeleteEvent(evt event.DeleteEvent) {
	m.Map.Delete(m.getKey(evt.Meta))
}

func (m *eventLockMap) PutEventsOn(eventChan chan event.GenericEvent) {
	m.Map.Range(func(key, value interface{}) bool {
		eventChan <- value.(event.GenericEvent)
		return true
	})
}

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	opt := options.GetOptions()
	return &ReconcileMysqlCluster{
		Client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		recorder:  mgr.GetRecorder(controllerName),
		orcClient: orc.NewFromURI(opt.OrchestratorURI),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlCluster. just for add and delete events
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlCluster{}}, &handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			if evt.Meta == nil {
				log.Error(nil, "CreateEvent received with no metadata", "CreateEvent", evt)
				return
			}

			clustersMap.CreateEvent(evt)
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if evt.Meta == nil {
				log.Error(nil, "DeleteEvent received with no metadata", "DeleteEvent", evt)
				return
			}

			clustersMap.DeleteEvent(evt)
		},
	})
	if err != nil {
		return err
	}

	// create source channel that listen for events on events chan
	events := make(chan event.GenericEvent)
	chSource := source.Channel{Source: events}

	// inject stop channel into source
	// this is done due to: https://github.com/kubernetes-sigs/controller-runtime/issues/103
	if err = chSource.InjectStopChannel(stop.Channel); err != nil {
		return err
	}

	// watch for events on channel `events`
	if err = c.Watch(&chSource, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// start the go routine that dispatch events
	go func() {
		for {
			select {
			case <-stop.Channel:
				return
			case <-time.After(reconcileTimePeriod):
				// write all clusters to envents chan to be processed\
				clustersMap.PutEventsOn(events)
			}
		}
	}()

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlCluster{}

// ReconcileMysqlCluster reconciles a MysqlCluster object
type ReconcileMysqlCluster struct {
	client.Client
	scheme    *runtime.Scheme
	recorder  record.EventRecorder
	orcClient orc.Interface
}

// Reconcile reconcile for each mysql cluster state from orchestrator to
// cluster, this reconcile is triggered in a loop.
//
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
func (r *ReconcileMysqlCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the MysqlCluster instance
	cluster := &mysqlv1alpha1.MysqlCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("Reconciling cluster: %s/%s", cluster.Name, cluster.Namespace))

	defer func() {
		if sErr := r.Status().Update(context.TODO(), cluster); sErr != nil {
			log.Error(sErr, "failed to update cluster status", "cluster", cluster)
		}
	}()

	ou := NewOrcUpdater(cluster, r.recorder, r.orcClient)
	if err := ou.Sync(); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
