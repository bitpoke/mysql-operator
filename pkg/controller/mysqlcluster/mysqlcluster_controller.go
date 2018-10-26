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
	"reflect"

	"github.com/presslabs/controller-util/syncer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	cleaner "github.com/presslabs/mysql-operator/pkg/controller/mysqlcluster/internal/cleaner"
	"github.com/presslabs/mysql-operator/pkg/controller/mysqlcluster/internal/syncer"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var log = logf.Log.WithName(controllerName)

const controllerName = "controller.mysqlcluster"

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

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &policyv1beta1.PodDisruptionBudget{}}, &handler.EnqueueRequestForOwner{
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
// nolint: gocyclo
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;services;events;jobs;pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlclusters;mysqlclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
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

	// Set defaults on cluster
	r.scheme.Default(cluster)
	wrapcluster.NewMysqlClusterWrapper(cluster).SetDefaults(r.opt)

	status := *cluster.Status.DeepCopy()
	defer func() {
		if !reflect.DeepEqual(status, cluster.Status) {
			sErr := r.Status().Update(context.TODO(), cluster)
			if sErr != nil {
				log.Error(sErr, "failed to update cluster status", "cluster", cluster)
			}
		}
	}()

	configMapSyncer := mysqlcluster.NewConfigMapSyncer(r.Client, r.scheme, cluster)
	if err := syncer.Sync(context.TODO(), configMapSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	secretSyncer := mysqlcluster.NewSecretSyncer(r.Client, r.scheme, cluster, r.opt)
	if err := syncer.Sync(context.TODO(), secretSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	configMapResourceVersion := configMapSyncer.GetObject().(*corev1.ConfigMap).ResourceVersion
	secretResourceVersion := secretSyncer.GetObject().(*corev1.Secret).ResourceVersion

	// run the syncers for services, pdb and statefulset
	syncers := []syncer.Interface{
		mysqlcluster.NewHeadlessSVCSyncer(r.Client, r.scheme, cluster),
		mysqlcluster.NewMasterSVCSyncer(r.Client, r.scheme, cluster),
		mysqlcluster.NewHealthySVCSyncer(r.Client, r.scheme, cluster),

		mysqlcluster.NewStatefulSetSyncer(r.Client, r.scheme, cluster, configMapResourceVersion, secretResourceVersion, r.opt),
	}

	if len(cluster.Spec.MinAvailable) != 0 {
		syncers = append(syncers, mysqlcluster.NewPDBSyncer(r.Client, r.scheme, cluster))
	}

	syncers = append(syncers, r.getPodSyncers(cluster)...)

	// add pods syncers for every node status
	for _, sync := range syncers {
		if err := syncer.Sync(context.TODO(), sync, r.recorder); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Perform any cleanup
	pvcCleaner := cleaner.NewPvcCleaner(cluster, r.opt)
	err = pvcCleaner.Run(context.TODO(), r.Client, r.scheme, r.recorder)

	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// getPodSyncers returns a list of syncers for every pod of the cluster. The
// list is sorted by node roles, first are replicas and the last is the master
// pod syncer. We need to have replicas first to avoid having two pods with
// master label in the same time. This can happen for a small period of time
// when master changes.
func (r *ReconcileMysqlCluster) getPodSyncers(cluster *mysqlv1alpha1.MysqlCluster) []syncer.Interface {
	syncers := []syncer.Interface{}

	// add replica syncers, those should be the first in this list.
	for _, ns := range cluster.Status.Nodes {
		if !getCondAsBool(&ns, mysqlv1alpha1.NodeConditionMaster) {
			syncers = append(syncers, mysqlcluster.NewPodSyncer(r.Client, r.scheme, cluster, ns.Name))
		}
	}

	// add master syncers, this should be the last, and should be only one
	for _, ns := range cluster.Status.Nodes {
		if getCondAsBool(&ns, mysqlv1alpha1.NodeConditionMaster) {
			syncers = append(syncers, mysqlcluster.NewPodSyncer(r.Client, r.scheme, cluster, ns.Name))
		}
	}

	return syncers

}

func getCondAsBool(status *mysqlv1alpha1.NodeStatus, cond mysqlv1alpha1.NodeConditionType) bool {
	index, exists := wrapcluster.GetNodeConditionIndex(status, cond)
	return exists && status.Conditions[index].Status == corev1.ConditionTrue
}
