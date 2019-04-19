/*
Copyright 2019 Pressinfra SRL

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

package node

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strconv"
	"strings"
	"time"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

var log = logf.Log.WithName(controllerName)

const controllerName = "controller.mysqlNode"

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysqlNode{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetRecorder(controllerName),
		opt:      options.GetOptions(),
	}
}

func isOwnedByMySQL(meta metav1.Object) bool {
	if meta == nil {
		return false
	}

	// TODO: add more checks here
	labels := meta.GetLabels()
	if val, ok := labels["app.kubernetes.io/managed-by"]; ok {
		return val == "mysql.presslabs.org"
	}

	return false
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlCluster
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(evt event.CreateEvent) bool {
			return isOwnedByMySQL(evt.Meta)
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return isOwnedByMySQL(evt.MetaNew)
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			return isOwnedByMySQL(evt.Meta)
		},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlNode{}

// ReconcileMysqlNode reconciles a MysqlCluster object
type ReconcileMysqlNode struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options
}

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
func (r *ReconcileMysqlNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), request.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Info("syncing MySQL Node", "pod", request.NamespacedName.String())

	if isInitialized(pod) {
		// if pod is initialized then don't do anything
		log.Info("pod is initialized", "pod", pod.Spec.Hostname)
		return reconcile.Result{}, nil
	}

	cluster, err := r.getNodeCluster(pod)
	if err != nil {
		return reconcile.Result{}, err
	}

	var sql *nodeSQLRunner
	sql, err = r.getMySQLConnection(cluster, pod)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.initializeMySQL(sql, cluster)
	if err != nil {
		updatePodStatusCondition(pod, mysqlcluster.NodeInitializedConditionType,
			corev1.ConditionFalse, "mysqlInitializationFailed", err.Error())

		if uErr := r.updatePod(pod); uErr != nil {
			return reconcile.Result{}, uErr
		}

		return reconcile.Result{}, err
	}

	updatePodStatusCondition(pod, mysqlcluster.NodeInitializedConditionType,
		corev1.ConditionTrue, "mysqlInitializationSucceeded", "success")

	return reconcile.Result{}, r.updatePod(pod)
}

func (r *ReconcileMysqlNode) initializeMySQL(sql *nodeSQLRunner, cluster *mysqlcluster.MysqlCluster) error {
	// wait for mysql to be ready
	if err := sql.Wait(); err != nil {
		return err
	}

	if err := sql.DisableSuperReadOnly(); err != nil {
		return err
	}

	// is slave node
	if cluster.GetMasterHost() != sql.Host() {
		log.Info("configure pod as slave", "pod", sql.Host(), "master", cluster.GetMasterHost())
		if err := sql.SetGtidPurged(); err != nil {
			return err
		}

		if err := sql.ChangeMasterTo(cluster.GetMasterHost()); err != nil {
			return err
		}
		// TODO: check if node should be set read-only or not (super)
		// if err := sql.SetReadOnly(); err != nil {
		// 	return err
		// }
	}

	if err := sql.MarkConfigurationDone(); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileMysqlNode) nodeIndex(pod *corev1.Pod) (int, error) {
	re := regexp.MustCompile(`^[\w-]+-mysql-(\d*)$`)
	indexStrs := re.FindStringSubmatch(pod.Name)
	if len(indexStrs) != 2 {
		return -1, fmt.Errorf("pod name can't be parsed")
	}

	index, err := strconv.Atoi(indexStrs[1])
	if err != nil {
		return -1, err
	}

	return index, nil
}

// getNodeCluster returns the node related MySQL cluster
func (r *ReconcileMysqlNode) getNodeCluster(pod *corev1.Pod) (*mysqlcluster.MysqlCluster, error) {
	index, err := r.nodeIndex(pod)
	if err != nil {
		return nil, err
	}
	cName := strings.TrimSuffix(pod.Name, fmt.Sprintf("-mysql-%d", index))
	// TODO: remove this comment
	// re := regexp.MustCompile(`^([\w-]+)-mysql-\d*$`)
	// indexStrs := re.FindStringSubmatch(pod.Name)
	// if len(indexStrs) != 2 {
	//	 return nil, fmt.Errorf("pod name can't be parsed")
	// }
	// cName = indexStrs[1]
	clusterKey := types.NamespacedName{
		Name:      cName,
		Namespace: pod.Namespace,
	}
	cluster := mysqlcluster.New(&api.MysqlCluster{})
	err = r.Get(context.TODO(), clusterKey, cluster.Unwrap())
	return cluster, err
}

// getMySQLConnectionString returns the DSN that contains credentials to connect to given pod from a MySQL cluster
func (r *ReconcileMysqlNode) getMySQLConnection(cluster *mysqlcluster.MysqlCluster, pod *corev1.Pod) (*nodeSQLRunner, error) {
	host := fmt.Sprintf("%s.%s.%s", pod.Spec.Hostname,
		cluster.GetNameForResource(mysqlcluster.HeadlessSVC), pod.Namespace)

	secretKey := types.NamespacedName{
		Name:      cluster.GetNameForResource(mysqlcluster.Secret),
		Namespace: cluster.Namespace,
	}
	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), secretKey, secret); err != nil {
		return nil, err
	}

	user := string(secret.Data["OPERATOR_USER"])
	pass := string(secret.Data["OPERATOR_PASSWORD"])
	repU := string(secret.Data["REPLICATION_USER"])
	repP := string(secret.Data["REPLICATION_PASSWORD"])
	if notZero(user, pass, repU, repP) {
		return nil, fmt.Errorf("operator user or password not set into secret")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		user, pass, host, constants.MysqlPort,
	)

	return newNodeConn(dsn, host, repU, repP), nil
}

func (r *ReconcileMysqlNode) updatePod(pod *corev1.Pod) error {
	return r.Update(context.TODO(), pod)
}

func isInitialized(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == mysqlcluster.NodeInitializedConditionType {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func notZero(ss ...string) bool {
	zero := false
	for _, s := range ss {
		zero = zero || len(s) == 0
	}
	return zero
}

func updatePodStatusCondition(pod *corev1.Pod, condType corev1.PodConditionType,
	status corev1.ConditionStatus, reason, msg string) {
	newCondition := corev1.PodCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(pod.Status.Conditions) == 0 {
		newCondition.LastTransitionTime = metav1.NewTime(t)
		pod.Status.Conditions = []corev1.PodCondition{newCondition}
	} else {
		if i, exist := podCondIndex(pod, condType); exist {
			cond := pod.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			pod.Status.Conditions[i] = newCondition
		} else {
			newCondition.LastTransitionTime = metav1.NewTime(t)
			pod.Status.Conditions = append(pod.Status.Conditions, newCondition)
		}
	}
}

func podCondIndex(p *corev1.Pod, condType corev1.PodConditionType) (int, bool) {
	for i, cond := range p.Status.Conditions {
		if cond.Type == condType {
			return i, true
		}
	}

	return 0, false
}
