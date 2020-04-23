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

package mysqluser

import (
	"context"
	"errors"
	"fmt"
	"github.com/presslabs/mysql-operator/pkg/internal/mysql"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	logf "github.com/presslabs/controller-util/log"
	"github.com/presslabs/controller-util/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqluser"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	controllerName = "mysql-user"
	userFinalizer  = "finalizers.mysql.presslabs.org/created-in-mysql"
)

var log = logf.Log.WithName("controller.mysql-user")

// ReconcileMySQLUser reconciles a MySQLUser object
type ReconcileMySQLUser struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options

	// mysql query runner
	mysql.QueryRunner
}

// check for reconciler to implement reconciler.Reconciler interface
var _ reconcile.Reconciler = &ReconcileMySQLUser{}

// Reconcile reads that state of the cluster for a MySQLUser object and makes changes based on the state read
// and what is in the MySQLUser.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlusers;mysqlusers/status,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileMySQLUser) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the MySQLUser instance
	user := mysqluser.Wrap(&mysqlv1alpha1.MySQLUser{})

	err := r.Get(ctx, request.NamespacedName, user.Unwrap())
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !r.opt.AllowCreateUsersAcrossNamespaces && user.Namespace != user.Spec.ClusterRef.Namespace {
		err = fmt.Errorf("cross namespace user creation is disabled")
		return reconcile.Result{}, err
	}

	if user.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileUser(ctx, user)
	}

	// The resource has been deleted
	if meta.HasFinalizer(&user.ObjectMeta, userFinalizer) {
		// Drop the user if the finalizer is still present
		if err := r.dropUserFromDB(user); err != nil {
			return reconcile.Result{}, err
		}

		meta.RemoveFinalizer(&user.ObjectMeta, userFinalizer)

		if err := r.Update(ctx, user.Unwrap()); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileMySQLUser) reconcileUser(ctx context.Context, user *mysqluser.MySQLUser) (reconcile.Result, error) {
	log.Info("creating MySQL user", "name", user.Name, "database", user.Spec.User)
	resourceHasChanges := false

	// Set the user provisioning status to not ready if necessary
	_, exists := user.ConditionExists(mysqlv1alpha1.MySQLUserReady)
	if !exists {
		_, resourceHasChanges = user.UpdateStatusCondition(
			mysqlv1alpha1.MySQLUserReady, corev1.ConditionFalse,
			mysqluser.ProvisionInProgressReason, "The user provisioning has started.",
		)
	}

	// Add the user finalizer if it doesn't exist
	if !meta.HasFinalizer(&user.ObjectMeta, userFinalizer) {
		meta.AddFinalizer(&user.ObjectMeta, userFinalizer)

		resourceHasChanges = true
	}

	if resourceHasChanges {
		if err := r.Update(ctx, user.Unwrap()); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Reconcile the user
	err := r.reconcileUserInDB(ctx, user)

	var conditionUpdated bool

	// Update the status according to the result
	if err == nil {
		_, conditionUpdated = user.UpdateStatusCondition(
			mysqlv1alpha1.MySQLUserReady, corev1.ConditionTrue,
			mysqluser.ProvisionSucceededReason, "The user provisioning has succeeded.",
		)
	} else {
		_, conditionUpdated = user.UpdateStatusCondition(
			mysqlv1alpha1.MySQLUserReady, corev1.ConditionFalse,
			mysqluser.ProvisionFailedReason, "The user provisioning has failed.",
		)
	}

	if conditionUpdated {
		statusUpdateErr := r.Update(ctx, user.Unwrap())
		if statusUpdateErr != nil {
			return reconcile.Result{}, statusUpdateErr
		}
	}

	return reconcile.Result{}, err
}

func (r *ReconcileMySQLUser) reconcileUserInDB(ctx context.Context, user *mysqluser.MySQLUser) error {
	cfg, err := mysql.NewConfigFromClusterKey(r.Client, user.GetClusterKey(), r.QueryRunner)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{Name: user.Spec.Password.Name, Namespace: user.Namespace}

	if err := r.Get(ctx, secretKey, secret); err != nil {
		return err
	}

	password := string(secret.Data[user.Spec.Password.Key])
	if password == "" {
		return errors.New("the MySQL user's password must not be empty")
	}

	return mysql.CreateUserIfNotExists(cfg, user.Spec.User, password, user.Spec.AllowedHost,
		user.Spec.Permissions, user.Spec.AccountResourceLimits)
}

func (r *ReconcileMySQLUser) dropUserFromDB(user *mysqluser.MySQLUser) error {
	cfg, err := mysql.NewConfigFromClusterKey(r.Client, user.GetClusterKey(), r.QueryRunner)
	if apierrors.IsNotFound(err) {
		statusErr, ok := err.(*apierrors.StatusError)
		if ok && statusErr.Status().Details.Kind == "MysqlCluster" {
			// it seems the cluster is not to be found, so we assume it has been deleted
			return nil
		}
	}

	if err != nil {
		return err
	}

	return mysql.DropUser(cfg, user.Spec.User)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, qr mysql.QueryRunner) reconcile.Reconciler {
	return &ReconcileMySQLUser{
		Client:      mgr.GetClient(),
		scheme:      mgr.GetScheme(),
		recorder:    mgr.GetRecorder(controllerName),
		opt:         options.GetOptions(),
		QueryRunner: qr,
	}
}

// BuilderForManager creates the controller and returns a builder not yet registered to the manager
func add(mgr ctrl.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MySQLUser
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MySQLUser{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Add will register the controller to the manager
func Add(mgr ctrl.Manager) error {
	return add(mgr, newReconciler(mgr, mysql.StandardQueryRunner))
}
