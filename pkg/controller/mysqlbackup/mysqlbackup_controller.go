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

package mysqlbackup

import (
	"context"
	"fmt"
	"reflect"

	"github.com/presslabs/controller-util/syncer"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	backupSyncer "github.com/presslabs/mysql-operator/pkg/controller/mysqlbackup/internal/syncer"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	controllerName = "mysqlbackup-controller"
)

var log = logf.Log.WithName(controllerName)

// Add creates a new MysqlBackup Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysqlBackup{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetRecorder(controllerName),
		opt:      options.GetOptions(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlBackup
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlBackup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlBackup{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlBackup{}

// ReconcileMysqlBackup reconciles a MysqlBackup object
type ReconcileMysqlBackup struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options
}

// Reconcile reads that state of the cluster for a MysqlBackup object and makes changes based on the state read
// and what is in the MysqlBackup.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlbackups,verbs=get;list;watch;create;update;patch;delete
// nolint: gocyclo
func (r *ReconcileMysqlBackup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the MysqlBackup instance
	backup := &mysqlv1alpha1.MysqlBackup{}
	err := r.Get(context.TODO(), request.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.V(1).Info("reconcile backup", "backup", backup)

	// migrate old backups to the new version
	// TODO: remove this in version v0.3.0
	if backup.Spec.BackupURL == "" && backup.Spec.BackupURI == "" && backup.Status.Completed && len(backup.Status.BackupURI) > 0 {
		backup.Spec.BackupURL = backup.Status.BackupURI
		return reconcile.Result{}, r.Update(context.TODO(), backup)
	}

	savedBackup := backup.DeepCopy()
	if len(backup.Spec.ClusterName) == 0 {
		return reconcile.Result{}, fmt.Errorf("cluster name is not specified")
	}

	if backup.Status.Completed {
		// silence skip it
		log.V(1).Info("backup already completed", "name", backup.Name)
		return reconcile.Result{}, nil
	}

	clusterKey := types.NamespacedName{Name: backup.Spec.ClusterName, Namespace: backup.Namespace}
	cluster := &mysqlv1alpha1.MysqlCluster{}
	if err = r.Get(context.TODO(), clusterKey, cluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("cluster not found: %s", err)
	}

	wCluster := mysqlcluster.New(cluster)
	wBackup := mysqlbackup.New(backup)
	wBackup.SetDefaults(wCluster)

	jobSyncer := backupSyncer.NewJobSyncer(r.Client, r.scheme, wBackup, wCluster, r.opt)
	err = syncer.Sync(context.TODO(), jobSyncer, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	// update spec
	if !reflect.DeepEqual(savedBackup, backup) {
		if err = r.Update(context.TODO(), backup); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
