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

package orchestrator

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/go-test/deep"
	"github.com/presslabs/controller-util/syncer"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

const (
	eventNormal = "Normal"
	//eventWarning   = "Warning"
	controllerName = "controller.orchestrator"

	// OrchestratorFinalizer is set when the cluster is registered in
	// orchestrator and is removed when no nodes are in orchestrator
	OrchestratorFinalizer = "mysql.presslabs.org/registered-in-orchestrator"

	// OldOrchestratorFinalizer is the old finalizer name that is DEPRECATED
	OldOrchestratorFinalizer = "OrchestratorFinalizer"
)

var log = logf.Log.WithName(controllerName)

// reconcileTimePeriod represents the time in which a cluster should be reconciled
var reconcileTimePeriod = time.Second * 5

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	opt := options.GetOptions()
	orcClient := orc.NewFromURI(opt.OrchestratorURI)
	return add(mgr, newReconciler(mgr, orcClient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, orcClient orc.Interface) reconcile.Reconciler {
	return &ReconcileMysqlCluster{
		Client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		recorder:  mgr.GetRecorder(controllerName),
		orcClient: orcClient,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
// nolint: gocyclo
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// stores a mapping between cluster and its creation event,
	// so that we know what clusters to sync with orchestrator
	clusters := &sync.Map{}

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

			log.V(1).Info("register cluster in clusters list", "meta", evt.Meta)
			clusters.Store(getKey(evt.Meta), event.GenericEvent{ // nolint: megacheck
				Meta:   evt.Meta,
				Object: evt.Object,
			})
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if evt.Meta == nil {
				log.Error(nil, "DeleteEvent received with no metadata", "DeleteEvent", evt)
				return
			}

			log.V(1).Info("remove cluster from clusters list", "meta", evt.Meta)
			clusters.Delete(getKey(evt.Meta))
		},
	})
	if err != nil {
		return err
	}

	// create source channel that listen for events on events chan
	events := make(chan event.GenericEvent)
	chSource := source.Channel{Source: events}

	// watch for events on channel `events`
	if err = c.Watch(&chSource, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// create a runnable function that dispatches events to events channel
	// this runnableFunc is passed to the manager that starts it.
	var f manager.RunnableFunc = func(stop <-chan struct{}) error {
		for {
			select {
			case <-stop:
				return nil
			case <-time.After(reconcileTimePeriod):
				// write all clusters to events chan to be processed
				clusters.Range(func(key, value interface{}) bool {
					events <- value.(event.GenericEvent)
					return true
				})
			}
		}
	}

	err = mgr.Add(f)
	if err != nil {
		return err
	}

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
	cluster := mysqlcluster.New(&mysqlv1alpha1.MysqlCluster{})
	err := r.Get(context.TODO(), request.NamespacedName, cluster.Unwrap())
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			log.Info("cluster is deleted", "key", request.NamespacedName.String())
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// save old status
	status := *cluster.Status.DeepCopy()
	log.Info("reconciling cluster", "cluster", cluster)

	// this syncer mutates the cluster and updates it. Should be the first syncer
	finSyncer := newFinalizerSyncer(r.Client, r.scheme, cluster, r.orcClient)
	if err := syncer.Sync(context.TODO(), finSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	// Set defaults on cluster, should be set here because the syncer from above fetch a new copy of the cluster
	// Some filed like .spec.replicas can be nil and throw a panic error.
	// By setting defaults will ensure that all fields are set at least with a default value.
	r.scheme.Default(cluster.Unwrap())

	orcSyncer := NewOrcUpdater(cluster, r.recorder, r.orcClient)
	if err := syncer.Sync(context.TODO(), orcSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	// update cluster because newOrcUpdater syncer updates the .Status
	if !reflect.DeepEqual(status, cluster.Unwrap().Status) && cluster.DeletionTimestamp == nil {
		log.V(1).Info("update cluster", "diff", deep.Equal(status, cluster.Unwrap().Status))

		if sErr := r.Status().Update(context.TODO(), cluster.Unwrap()); sErr != nil {
			log.Error(sErr, "failed to update cluster status")
			return reconcile.Result{}, sErr
		}

	}

	return reconcile.Result{}, nil
}

// getKey returns a string that represents the key under which cluster is registered
func getKey(meta metav1.Object) string {
	return types.NamespacedName{
		Namespace: meta.GetNamespace(),
		Name:      meta.GetName(),
	}.String()
}
