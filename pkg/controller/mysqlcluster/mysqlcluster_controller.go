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

package mysqlcluster

import (
	"context"
	"log"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/syncers"
	"github.com/presslabs/mysql-operator/pkg/syncers/mysqlcluster"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysqlCluster{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mysqlcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlCluster
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by MysqlCluster - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlCluster{}

// ReconcileMysqlCluster reconciles a MysqlCluster object
type ReconcileMysqlCluster struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlclusters,verbs=get;list;watch;create;update;patch;delete
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

	// check for secretName to be specified
	if len(cluster.Spec.SecretName) != 0 {
		return reconcile.Result{}, fmt.Errorf("the spec.SecretName is empty")
	}

	// run the config syncers
	configSyncers = []syncers.Interface{
		mysqlcluster.NewConfigMapSyncer(cluster),
		mysqlcluster.NewSecretSyncer(cluster),
	}

	err := r.sync(configSyncers)
	if err != nil {
		return reconcile.Result{}, err
	}

	// get config map and secret revision to annotate the statefulset
	configRev, secretRev, err := r.getConfigAndSecretRevs(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// run the syncers for services, pdb and statefulset
	otherSyncers = []syncers.Interface{
		mysqlcluster.NewPDBSyncer(cluster),

		mysqlcluster.NewHeadlessSVCSyncer(cluster),
		mysqlcluster.NewHealthySVCSyncer(cluster),
		mysqlcluster.NewMasterSVCSyncer(cluster),

		mysqlcluster.NewStatefulSetSyncer(cluster, configRev, secretRev),
	}

	err := r.sync(otherSyncers)

	return reconcile.Result{}, nil
}

func (r *ReconcileMysqlCluster) sync(cluster *mysqlv1alpha1.MysqlCluster, syncers []syncers.Interface) error {
	for _, s := range syncers {
		existing := s.GetExistingObjectPlaceholder()

		// set owner reference on objects that requires owner reference
		syncFn := s.Sync
		if s.ShouldHaveOwnerReference() {
			syncFn = func(in runtime.Object) error {
				if err := controllerutil.SetControllerReference(cluster, in, r.scheme); err != nil {
					return err
				}
				return s.Sync(in)
			}
		}

		op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, existing, syncFn)
		reason := getErrorEventReason(existing, err)

		log.Info(fmt.Sprintf("%T %s/%s %s", existing, cluster.Namespace, cluster.Name, op))

		if err != nil {
			r.recorder.Eventf(wp, eventWarning, reason, "%T %s/%s failed syncing: %s", existing, cluster.Namespace, cluster.Name, err)
			return err
		}
		if op != controllerutil.OperationNoop {
			r.recorder.Eventf(wp, eventNormal, reason, "%T %s/%s %s successfully", existing, cluster.Namespace, cluster.Name, op)
		}
	}
	return nil
}

func (r *ReconcileMysqlCLuster) getConfigAndSecretRevs(cluster *mysqlv1alpha1.MysqlCluster) (cmRev string, secRev string, err error) {
	configMap := &core.ConfigMap{
		Name:      cluster.GetNameForResource(mysqlv1alpha1.ConfigMap),
		Namespace: cluster.Namespace,
	}
	err = r.Get(context.TODO(), configMap)
	if err != nil {
		return
	}

	secret := &core.Secret{
		Name:      cluster.Spec.SecretName,
		Namespace: cluster.Namespace,
	}
	err = r.Get(context.TODO(), secret)
	if err != nil {
		return
	}

	cmRev = configMap.ResourceVersion
	secRev = secret.ResourceVersion
}

func getErrorEventReason(obj runtime.Object, err error) string {
	op := "Updated"
	if err != nil {
		op = "Failed"
	}

	return fmt.Sprintf("%T%s", obj, op)
}
