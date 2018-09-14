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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
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
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/presslabs/controller-util/syncer"
	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/controller/mysqlcluster/internal/syncer/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var log = logf.Log.WithName(controllerName)

const controllerName = "mysqlcluster-controller"

const (
	eventNormal  = "Normal"
	eventWarning = "Warning"
)

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysqlCluster{
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

	// Watch for changes to MysqlCluster
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &core.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &core.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	// TODO watch for secret

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlCluster{}

// ReconcileMysqlCluster reconciles a MysqlCluster object
type ReconcileMysqlCluster struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options
}

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;services;events;jobs;pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;create;update;patch;delete
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
	log.Info("syncing cluster", "cluster", request.NamespacedName.String())

	// check for secretName to be specified
	if len(cluster.Spec.SecretName) == 0 {
		return reconcile.Result{}, fmt.Errorf("the spec.secretName is empty")
	}

	// Set defaults on cluster
	r.scheme.Default(cluster)
	wrapcluster.NewMysqlClusterWrapper(cluster).SetDefaults(r.opt)

	defer func() {
		sErr := r.Status().Update(context.TODO(), cluster)
		if sErr != nil {
			log.Error(sErr, "Failed to update cluster status!", "cluster", cluster)
		}
	}()

	configMapSyncer := mysqlcluster.NewConfigMapSyncer(cluster)
	err = syncer.Sync(context.TODO(), configMapSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	secretSyncer := mysqlcluster.NewSecretSyncer(cluster)
	err = syncer.Sync(context.TODO(), secretSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	// run the syncers for services, pdb and statefulset
	otherSyncers := []syncer.Interface{
		mysqlcluster.NewHeadlessSVCSyncer(cluster),
		mysqlcluster.NewMasterSVCSyncer(cluster),
		mysqlcluster.NewHealthySVCSyncer(cluster),

		mysqlcluster.NewStatefulSetSyncer(cluster, configMapSyncer.GetObject().(*core.ConfigMap).ResourceVersion, secretSyncer.GetObject().(*core.Secret).ResourceVersion, r.opt),
	}

	headlessSVCSyncer := mysqlcluster.NewHeadlessSVCSyncer(cluster)
	err = syncer.Sync(context.TODO(), headlessSVCSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	masterSVCSyncer := mysqlcluster.NewMasterSVCSyncer(cluster)
	err = syncer.Sync(context.TODO(), masterSVCSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	healthySVCSyncer := mysqlcluster.NewHealthySVCSyncer(cluster)
	err = syncer.Sync(context.TODO(), healthySVCSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	configResourceVersion, secretResourceVersion, err := r.getConfigAndSecretRevs(cluster)

	if err != nil {
		return reconcile.Result{}, err
	}

	statefulSetSyncer := mysqlcluster.NewStatefulSetSyncer(cluster, configResourceVersion, secretResourceVersion, r.opt)
	err = syncer.Sync(context.TODO(), statefulSetSyncer, r.Client, r.scheme, r.recorder)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(cluster.Spec.MinAvailable) != 0 {
		otherSyncers = append(otherSyncers, mysqlcluster.NewPDBSyncer(cluster))

		pdbSyncer := mysqlcluster.NewPDBSyncer(cluster)
		err = syncer.Sync(context.TODO(), pdbSyncer, r.Client, r.scheme, r.recorder)
		if err != nil {
			return reconcile.Result{}, err
		}

	}

	return reconcile.Result{}, nil
}

func (r *ReconcileMysqlCluster) sync(cluster *mysqlv1alpha1.MysqlCluster, syncers []syncer.Interface) error {
	for _, s := range syncers {
		existing := s.GetObject()

		// set owner reference on objects that requires owner reference
		syncFn := s.SyncFn
		if s.GetOwner() != nil {
			syncFn = func(in runtime.Object) error {
				// TODO: check this
				metaIn := in.(metav1.Object)
				if err := controllerutil.SetControllerReference(cluster, metaIn, r.scheme); err != nil {
					return err
				}
				return s.SyncFn(in)
			}
		}

		op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, existing, syncFn)
		reason := getErrorEventReason(existing, err)

		key := types.NamespacedName{
			Name:      cluster.GetName(),
			Namespace: cluster.GetNamespace(),
		}

		log.Info(string(op), "key", key.String(), "kind", existing.GetObjectKind().GroupVersionKind().Kind)

		if err != nil {
			r.recorder.Eventf(cluster, eventWarning, reason, "%T %s/%s failed syncing: %s", existing, cluster.Namespace, cluster.Name, err)
			return err
		}
		if op != controllerutil.OperationResultNone {
			r.recorder.Eventf(cluster, eventNormal, reason, "%T %s/%s %s successfully", existing, cluster.Namespace, cluster.Name, op)
		}
	}
	return nil
}

func (r *ReconcileMysqlCluster) getConfigAndSecretRevs(cluster *mysqlv1alpha1.MysqlCluster) (string, string, error) {
	configMap := &core.ConfigMap{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      cluster.GetNameForResource(mysqlv1alpha1.ConfigMap),
		Namespace: cluster.Namespace,
	}, configMap)
	if err != nil {
		return "", "", err
	}

	secret := &core.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      cluster.Spec.SecretName,
		Namespace: cluster.Namespace,
	}, secret)
	if err != nil {
		return "", "", err
	}

	return configMap.ResourceVersion, secret.ResourceVersion, nil
}

func getErrorEventReason(obj runtime.Object, err error) string {
	op := "Updated"
	if err != nil {
		op = "Failed"
	}

	return fmt.Sprintf("%T%s", obj, op)
}
