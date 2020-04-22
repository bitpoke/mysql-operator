/*
Copyright 2020 Pressinfra SRL.

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

package mysqldatabase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-test/deep"
	logf "github.com/presslabs/controller-util/log"
	utilmeta "github.com/presslabs/controller-util/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysql"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqldatabase"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	mysqlPreventDeletionFinalizer = "mysql-operator.presslabs.org/database"
	controllerName                = "mysql-database"
)

var log = logf.Log.WithName("controller.mysql-database")

// ReconcileMySQLDatabase reconciles a Wordpress object
type ReconcileMySQLDatabase struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options

	// mysql query runner
	mysql.SQLRunnerFactory
}

// check for reconciler to implement reconciler.Reconciler interface
var _ reconcile.Reconciler = &ReconcileMySQLDatabase{}

// Reconcile reads that state of the cluster for a Wordpress object and makes changes based on the state read
// and what is in the Wordpress.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqldatabases;mysqldatabases/status,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileMySQLDatabase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the MysqlDatabase instance
	db := mysqldatabase.Wrap(&mysqlv1alpha1.MysqlDatabase{})

	err := r.Get(ctx, request.NamespacedName, db.Unwrap())
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	oldDBStatus := db.DeepCopy().Status

	if !r.opt.AllowCrossNamespaceDatabases && db.Namespace != db.GetClusterKey().Namespace {
		err = fmt.Errorf("cross namespace database creation is disabled")
		return reconcile.Result{}, r.updateReadyCondition(ctx, oldDBStatus, db, err)
	}

	// Check if the resource is deleted
	if !db.ObjectMeta.DeletionTimestamp.IsZero() {
		err = r.deleteDatabase(ctx, db)
		if err != nil {
			return reconcile.Result{}, err
		}

		// remove finalizer
		utilmeta.RemoveFinalizer(&db.ObjectMeta, mysqlPreventDeletionFinalizer)

		// update resource to remove finalizer, no status
		return reconcile.Result{}, r.Update(ctx, db.Unwrap())
	}

	// reconcile database in mysql
	err = r.createDatabase(ctx, db)
	if err != nil {
		return reconcile.Result{}, r.updateReadyCondition(ctx, oldDBStatus, db, err)
	}

	// Add finalizer if needed
	if !utilmeta.HasFinalizer(&db.ObjectMeta, mysqlPreventDeletionFinalizer) {
		utilmeta.AddFinalizer(&db.ObjectMeta, mysqlPreventDeletionFinalizer)
		if uErr := r.Update(ctx, db.Unwrap()); uErr != nil {
			return reconcile.Result{}, uErr
		}
	}

	return reconcile.Result{}, r.updateReadyCondition(ctx, oldDBStatus, db, err)
}

func (r *ReconcileMySQLDatabase) deleteDatabase(ctx context.Context, db *mysqldatabase.Database) error {
	log.Info("deleting MySQL database", "name", db.Name, "database", db.Spec.Database)

	sql, close, err := r.SQLRunnerFactory(mysql.NewConfigFromClusterKey(r.Client, db.GetClusterKey()))
	if apierrors.IsNotFound(err) {
		// if the mysql cluster does not exists then we can safely assume that
		// the db is deleted so exist successfully
		statusErr, ok := err.(*apierrors.StatusError)
		if ok && statusErr.Status().Details.Kind == "MysqlCluster" {
			// it seems the cluster is not to be found, so we assume it has been deleted
			return nil
		}
	} else if err != nil {
		return err
	}

	defer close()

	// Remove database from MySQL then remove finalizer
	if err = mysql.DropDatabase(ctx, sql, db.Spec.Database); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileMySQLDatabase) createDatabase(ctx context.Context, db *mysqldatabase.Database) error {
	log.Info("creating MySQL database", "name", db.Name, "database", db.Spec.Database)

	sql, close, err := r.SQLRunnerFactory(mysql.NewConfigFromClusterKey(r, db.GetClusterKey()))
	if err != nil {
		return err
	}

	defer close()

	// Create database if does not exists
	return mysql.CreateDatabaseIfNotExists(ctx, sql, db.Spec.Database)
}

func (r *ReconcileMySQLDatabase) updateReadyCondition(
	ctx context.Context, oldDBStatus mysqlv1alpha1.MysqlDatabaseStatus, db *mysqldatabase.Database, err error) error {
	if err == nil {
		db.UpdateCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionTrue, mysqldatabase.ProvisionSucceeded, "Database successfully created.")
	} else {
		db.UpdateCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionFalse, mysqldatabase.ProvisionFailed, err.Error())
	}

	if !reflect.DeepEqual(oldDBStatus, db.Status) {
		log.V(1).Info("update MySQL database status", "diff", deep.Equal(oldDBStatus, db.Status))

		if uErr := r.Status().Update(ctx, db.Unwrap()); uErr != nil {
			return uErr
		}
	}

	// return the original error
	return err
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, sqlFactory mysql.SQLRunnerFactory) reconcile.Reconciler {
	return &ReconcileMySQLDatabase{
		Client:           mgr.GetClient(),
		scheme:           mgr.GetScheme(),
		recorder:         mgr.GetEventRecorderFor(controllerName),
		opt:              options.GetOptions(),
		SQLRunnerFactory: sqlFactory,
	}
}

// BuilderForManager creates the controller and returns a builder not yet registered to the manager
func add(mgr ctrl.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlUser
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlDatabase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Add will register the controller to the manager
func Add(mgr ctrl.Manager) error {
	return add(mgr, newReconciler(mgr, mysql.NewSQLRunner))
}
