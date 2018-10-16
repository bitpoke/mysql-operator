/*
Copyright 2018 Platform9, Inc

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

package mysqlpvc

import (
	"context"
	"errors"
	"time"

	"github.com/presslabs/mysql-operator/pkg/options"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "controller.mysqlpvc"
	pvcFinalizer   = "pf9.io/MysqlPvcFinalizer"
)

var log = logf.Log.WithName(controllerName)

// PvcReconciler manages lifetime of PVCs created by MysqlCluster with finalizers
type PvcReconciler struct {
	client.Client
	scheme *runtime.Scheme
	opt    *options.Options
}

func newPvcReconciler(mgr manager.Manager) *PvcReconciler {
	return &PvcReconciler{Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		opt:    options.GetOptions(),
	}
}

// Add creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newPvcReconciler(mgr))
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PVC
	return c.Watch(&source.Kind{Type: &core.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{})
}

var _ reconcile.Reconciler = &PvcReconciler{}

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// nolint: gocyclo
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
func (c *PvcReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch pvc instance
	name := request.NamespacedName.String()
	pvc := &core.PersistentVolumeClaim{}
	err := c.Get(context.TODO(), request.NamespacedName, pvc)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			log.V(2).Info("not found", "pvc", name)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "error syncing", "pvc", name)
		return reconcile.Result{}, err
	}

	ownerRefs := pvc.GetObjectMeta().GetOwnerReferences()
	if ownerRefs == nil || len(ownerRefs) != 1 {
		log.V(4).Info("does not have owner reference", "pvc", name)
		return reconcile.Result{}, nil
	}

	isInteresting := false

	for _, ownerRef := range ownerRefs {
		if ownerRef.Kind == "MysqlCluster" {
			isInteresting = true
			break
		}
	}

	if !isInteresting {
		log.V(4).Info("does not have correct owner reference", "pvc", name)
		return reconcile.Result{}, nil
	}

	log.Info("syncing", "pvc", request.NamespacedName.String())

	// Add finalizer, if not present
	if pvc.DeletionTimestamp == nil {
		log.V(4).Info("adding finalizer", "pvc", name)
		addFinalizer(pvc, pvcFinalizer)
	} else {
		// If PVC was deleted, wait for cleanupGracePeriod and then remove finalizer
		cleanupGracePeriod := options.GetOptions().CleanupGracePeriod
		elapsed := time.Since(pvc.DeletionTimestamp.Time)
		if elapsed > cleanupGracePeriod {
			log.V(4).Info("removing finalizer", "pvc", name)
			removeFinalizer(pvc, pvcFinalizer)
		} else {
			log.V(4).Info("remaining", "time", cleanupGracePeriod-elapsed)
			return reconcile.Result{}, errors.New("Waiting for grace period to expire")
		}
	}

	// Update original pvc
	err = c.Update(context.TODO(), pvc)

	if err != nil {
		log.Error(err, "error updating", "pvc", name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func addFinalizer(in *core.PersistentVolumeClaim, finalizer string) {
	for _, f := range in.Finalizers {
		if f == finalizer {
			// Already exists
			return
		}
	}

	in.Finalizers = append(in.Finalizers, finalizer)
}

func removeFinalizer(in *core.PersistentVolumeClaim, finalizer string) {
	var (
		index int
		f     string
	)
	for index, f = range in.Finalizers {
		if f == finalizer {
			break
		}
	}

	in.Finalizers = append(in.Finalizers[:index], in.Finalizers[index+1:]...)
}
