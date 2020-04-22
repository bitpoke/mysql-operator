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
	"reflect"
	"time"

	"github.com/go-test/deep"
	logf "github.com/presslabs/controller-util/log"
	"github.com/presslabs/controller-util/meta"
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
	"github.com/presslabs/mysql-operator/pkg/internal/mysqluser"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	controllerName = "mysql-user"
	userFinalizer  = "mysql-operator.presslabs.org/user"
)

var log = logf.Log.WithName("controller.mysql-user")

// ReconcileMySQLUser reconciles a MysqlUser object
type ReconcileMySQLUser struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options

	// mysql query runner
	mysql.SQLRunnerFactory
}

// check for reconciler to implement reconciler.Reconciler interface
var _ reconcile.Reconciler = &ReconcileMySQLUser{}

// Reconcile reads that state of the cluster for a MysqlUser object and makes changes based on the state read
// and what is in the MysqlUser.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlusers;mysqlusers/status,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileMySQLUser) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the MysqlUser instance
	user := mysqluser.Wrap(&mysqlv1alpha1.MysqlUser{})

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

	if !r.opt.AllowCrossNamespaceUsers && user.Namespace != user.GetClusterKey().Namespace {
		return reconcile.Result{}, errors.New("cross namespace user creation is disabled")
	}

	oldStatus := user.Status.DeepCopy()

	// if the user has been deleted then remove it from mysql cluster
	if !user.ObjectMeta.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, r.removeUser(ctx, user)
	}

	// get the actual state from mysql cluster
	//	usErr := r.updateUserStatusFromCluster(ctx, user)
	//	if err := r.updateStatusAndErr(ctx, user, oldStatus, usErr); err != nil {
	//		return reconcile.Result{}, err
	//	}

	// write the desired status into mysql cluster
	ruErr := r.reconcileUserInCluster(ctx, user)
	if err := r.updateStatusAndErr(ctx, user, oldStatus, ruErr); err != nil {
		return reconcile.Result{}, err
	}

	// enqueue the resource again after to keep the resource up to date in mysql
	// in case is changed directly into mysql
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: 2 * time.Minute,
	}, nil
}

func (r *ReconcileMySQLUser) removeUser(ctx context.Context, user *mysqluser.MySQLUser) error {
	// The resource has been deleted
	if meta.HasFinalizer(&user.ObjectMeta, userFinalizer) {
		// Drop the user if the finalizer is still present
		if err := r.dropUserFromDB(ctx, user); err != nil {
			return err
		}

		meta.RemoveFinalizer(&user.ObjectMeta, userFinalizer)

		// update resource so it will remove the finalizer
		if err := r.Update(ctx, user.Unwrap()); err != nil {
			return err
		}
	}
	return nil
}

//func (r *ReconcileMySQLUser) updateUserStatusFromCluster(ctx context.Context, user *mysqluser.MysqlUser) (err error) {
//	// catch the error and set the failed status
//	defer setFailedStatus(&err, user)
//
//	return nil
//}

func (r *ReconcileMySQLUser) reconcileUserInCluster(ctx context.Context, user *mysqluser.MySQLUser) (err error) {
	// catch the error and set the failed status
	defer setFailedStatus(&err, user)

	// Reconcile the user into mysql
	if err = r.reconcileUserInDB(ctx, user); err != nil {
		return
	}

	// add finalizer if is not added on the resource
	if !meta.HasFinalizer(&user.ObjectMeta, userFinalizer) {
		meta.AddFinalizer(&user.ObjectMeta, userFinalizer)
		if err = r.Update(ctx, user.Unwrap()); err != nil {
			return
		}
	}

	// update status for allowedHosts if needed, mark that status need to be updated
	if !reflect.DeepEqual(user.Status.AllowedHosts, user.Spec.AllowedHosts) {
		user.Status.AllowedHosts = user.Spec.AllowedHosts
	}

	// Update the status according to the result
	user.UpdateStatusCondition(
		mysqlv1alpha1.MySQLUserReady, corev1.ConditionTrue,
		mysqluser.ProvisionSucceededReason, "The user provisioning has succeeded.",
	)

	return
}

func (r *ReconcileMySQLUser) reconcileUserInDB(ctx context.Context, user *mysqluser.MySQLUser) error {
	sql, closeConn, err := r.SQLRunnerFactory(mysql.NewConfigFromClusterKey(r.Client, user.GetClusterKey()))
	if err != nil {
		return err
	}
	defer closeConn()

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{Name: user.Spec.Password.Name, Namespace: user.Namespace}

	if err := r.Get(ctx, secretKey, secret); err != nil {
		return err
	}

	password := string(secret.Data[user.Spec.Password.Key])
	if password == "" {
		return errors.New("the MySQL user's password must not be empty")
	}

	// create/ update user in database
	log.Info("creating mysql user", "key", user.MysqlUser, "database", user.Spec.User)
	if err := mysql.CreateUserIfNotExists(ctx, sql, user.Spec.User, password, user.Spec.AllowedHosts,
		user.Spec.Permissions, user.Spec.ResourceLimits); err != nil {
		return err
	}

	// remove allowed hosts for user
	toRemove := stringDiffIn(user.Status.AllowedHosts, user.Spec.AllowedHosts)
	for _, host := range toRemove {
		if err := mysql.DropUser(ctx, sql, user.Spec.User, &host); err != nil {
			return err
		}
	}

	return nil
}

func stringDiffIn(actual, desired []string) []string {
	diff := []string{}
	for _, aStr := range actual {
		// if is not in the desired list remove it
		if _, exists := stringIn(aStr, desired); !exists {
			diff = append(diff, aStr)
		}
	}

	return diff
}

func stringIn(str string, strs []string) (int, bool) {
	for i, s := range strs {
		if s == str {
			return i, true
		}
	}
	return 0, false
}

func (r *ReconcileMySQLUser) dropUserFromDB(ctx context.Context, user *mysqluser.MySQLUser) error {
	sql, closeConn, err := r.SQLRunnerFactory(mysql.NewConfigFromClusterKey(r.Client, user.GetClusterKey()))
	defer closeConn()
	if apierrors.IsNotFound(err) {
		// if the mysql cluster does not exists then we can safely assume that
		// the user is deleted so exist successfully
		statusErr, ok := err.(*apierrors.StatusError)
		if ok && statusErr.Status().Details.Kind == "MysqlCluster" {
			// it seems the cluster is not to be found, so we assume it has been deleted
			return nil
		}
	}

	if err != nil {
		return err
	}

	return mysql.DropUser(ctx, sql, user.Spec.User, nil)
}

func (r *ReconcileMySQLUser) updateStatusAndErr(ctx context.Context, user *mysqluser.MySQLUser, oldStatus *mysqlv1alpha1.MysqlUserStatus, prevErr error) error {
	if !reflect.DeepEqual(oldStatus, &user.Status) {
		log.V(1).Info("update mysql user status", "key", user.MysqlUser, "diff", deep.Equal(oldStatus, &user.Status))

		if err := r.Status().Update(ctx, user.Unwrap()); err != nil {
			if prevErr != nil {
				return fmt.Errorf("failed to update status: %s, previous error was: %s", err, prevErr)
			}

			return err
		}
	}

	return prevErr
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, sqlFactory mysql.SQLRunnerFactory) reconcile.Reconciler {
	return &ReconcileMySQLUser{
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
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlUser{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Add will register the controller to the manager
func Add(mgr ctrl.Manager) error {
	return add(mgr, newReconciler(mgr, mysql.NewSQLRunner))
}

func setFailedStatus(err *error, user *mysqluser.MySQLUser) {
	if *err != nil {
		user.UpdateStatusCondition(
			mysqlv1alpha1.MySQLUserReady, corev1.ConditionFalse,
			mysqluser.ProvisionFailedReason, fmt.Sprintf("The user provisioning has failed: %s", *err),
		)
	}
}
