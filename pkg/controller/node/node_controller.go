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
	"regexp"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

var log = logf.Log.WithName(controllerName)

const controllerName = "controller.mysqlNode"

// mysqlReconciliationTimeout the time that should last a reconciliation (this is used as a MySQL timout too)
const mysqlReconciliationTimeout = 10 * time.Second

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, newNodeConn))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, sqlI sqlFactoryFunc) reconcile.Reconciler {
	return &ReconcileMysqlNode{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		recorder:   mgr.GetRecorder(controllerName),
		opt:        options.GetOptions(),
		sqlFactory: sqlI,
	}
}

func isOwnedByMySQL(meta metav1.Object) bool {
	if meta == nil {
		return false
	}

	labels := meta.GetLabels()
	if val, ok := labels["app.kubernetes.io/managed-by"]; ok {
		return val == "mysql.presslabs.org"
	}

	return false
}

func isInitialized(obj runtime.Object) bool {
	pod := obj.(*corev1.Pod)

	for _, cond := range pod.Status.Conditions {
		if cond.Type == mysqlcluster.NodeInitializedConditionType {
			return cond.Status == corev1.ConditionTrue
		}
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
			return isOwnedByMySQL(evt.Meta) && !isInitialized(evt.Object)
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return isOwnedByMySQL(evt.MetaNew) && !isInitialized(evt.ObjectNew)
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			return false
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

	sqlFactory sqlFactoryFunc
}

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileMysqlNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), mysqlReconciliationTimeout)
	defer cancel()

	pod := &corev1.Pod{}
	err := r.Get(ctx, request.NamespacedName, pod)
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

	// try to get the related MySQL Cluster for current node
	var cluster *mysqlcluster.MysqlCluster
	cluster, err = r.getNodeCluster(ctx, pod)
	if err != nil {
		log.Info("cluster is not found")
		return reconcile.Result{}, err
	}

	// if cluster is deleted then don't do anything
	if cluster.DeletionTimestamp != nil {
		log.Info("cluster is deleted nothing to do", "pod", pod.Spec.Hostname)
		return reconcile.Result{}, nil
	}

	// if it's a old version cluster then don't do anything
	if shouldUpdateToVersion(cluster, 300) {
		// if the cluster is upgraded then set on the cluster an annotations that skips the GTID configuration
		// TODO: this should be removed in the next versions
		cluster.Annotations["mysql.presslabs.org/SkipGTIDPurged"] = "true"
		return reconcile.Result{}, r.Update(ctx, cluster.Unwrap())
	}

	// get cluster credentials from k8s secret, like replication and operator credentials
	var creds *credentials
	if creds, err = r.getCredsSecret(ctx, cluster); err != nil {
		return reconcile.Result{}, err
	}

	// initialize SQL interface
	sql := r.getMySQLConnection(cluster, pod, creds)

	// run the initializer, this will connect to MySQL server and run init queries
	if err = r.initializeMySQL(ctx, sql, cluster, creds); err != nil {
		// initialization failed, mark node as not yet initialized (by updating pod init condition)
		updatePodStatusCondition(pod, mysqlcluster.NodeInitializedConditionType,
			corev1.ConditionFalse, "mysqlInitializationFailed", err.Error())

		if uErr := r.updatePod(ctx, pod); uErr != nil {
			return reconcile.Result{}, uErr
		}

		return reconcile.Result{}, err
	}

	// initialization complete
	updatePodStatusCondition(pod, mysqlcluster.NodeInitializedConditionType,
		corev1.ConditionTrue, "mysqlInitializationSucceeded", "success")

	return reconcile.Result{}, r.updatePod(ctx, pod)
}

func (r *ReconcileMysqlNode) initializeMySQL(ctx context.Context, sql SQLInterface, cluster *mysqlcluster.MysqlCluster, c *credentials) error {
	// wait for mysql to be ready
	if err := sql.Wait(ctx); err != nil {
		return err
	}

	// check if MySQL was configured before to avoid multiple times reconfiguration
	if configured, err := sql.IsConfigured(ctx); err != nil {
		return err
	} else if configured {
		// already configured. For example this can be reached if the pod status update fails
		log.V(1).Info("MySQL is already configure - skip")
		return nil
	}

	// disable MySQL SUPER readonly to be able to modify settings in MySQL
	enableSuperReadOnly, err := sql.DisableSuperReadOnly(ctx)
	if err != nil {
		return err
	}
	defer enableSuperReadOnly()

	// is slave node?
	if cluster.GetMasterHost() != sql.Host() {
		log.Info("configure pod as slave", "pod", sql.Host(), "master", cluster.GetMasterHost())

		// check if the skip annotation is set on the cluster first
		if _, ok := cluster.Annotations["mysql.presslabs.org/SkipGTIDPurged"]; !ok {
			if err := sql.SetPurgedGTID(ctx); err != nil {
				return err
			}
		}

		if err := sql.ChangeMasterTo(ctx, cluster.GetMasterHost(), c.ReplicationUser, c.ReplicationPassword); err != nil {
			return err
		}
	}

	// write the configuration complete flag into MySQL, this will make the node ready
	if err := sql.MarkConfigurationDone(ctx); err != nil {
		return err
	}

	return nil
}

// getNodeCluster returns the node related MySQL cluster
func (r *ReconcileMysqlNode) getNodeCluster(ctx context.Context, pod *corev1.Pod) (*mysqlcluster.MysqlCluster, error) {
	re := regexp.MustCompile(`^([\w-]+)-mysql-\d*$`)
	indexStrs := re.FindStringSubmatch(pod.Name)
	if len(indexStrs) != 2 {
		return nil, fmt.Errorf("pod name can't be parsed")
	}
	cName := indexStrs[1]
	clusterKey := types.NamespacedName{
		Name:      cName,
		Namespace: pod.Namespace,
	}
	cluster := mysqlcluster.New(&api.MysqlCluster{})
	err := r.Get(ctx, clusterKey, cluster.Unwrap())
	return cluster, err
}

// getMySQLConnectionString returns the DSN that contains credentials to connect to given pod from a MySQL cluster
func (r *ReconcileMysqlNode) getMySQLConnection(cluster *mysqlcluster.MysqlCluster, pod *corev1.Pod, c *credentials) SQLInterface {
	host := fmt.Sprintf("%s.%s.%s", pod.Spec.Hostname,
		cluster.GetNameForResource(mysqlcluster.HeadlessSVC), pod.Namespace)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&multiStatements=true&interpolateParams=true",
		c.User, c.Password, host, constants.MysqlPort,
	)

	return r.sqlFactory(dsn, host)
}

type credentials struct {
	User     string
	Password string

	ReplicationUser     string
	ReplicationPassword string
}

func (r *ReconcileMysqlNode) getCredsSecret(ctx context.Context, cluster *mysqlcluster.MysqlCluster) (*credentials, error) {
	secretKey := types.NamespacedName{
		Name:      cluster.GetNameForResource(mysqlcluster.Secret),
		Namespace: cluster.Namespace,
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	creds := &credentials{
		User:                string(secret.Data["OPERATOR_USER"]),
		Password:            string(secret.Data["OPERATOR_PASSWORD"]),
		ReplicationUser:     string(secret.Data["REPLICATION_USER"]),
		ReplicationPassword: string(secret.Data["REPLICATION_PASSWORD"]),
	}

	return creds, creds.Validate()
}

func (r *ReconcileMysqlNode) updatePod(ctx context.Context, pod *corev1.Pod) error {
	return r.Status().Update(ctx, pod)
}

func (c *credentials) Validate() error {
	if anyIsEmpty(c.User, c.Password, c.ReplicationUser, c.ReplicationPassword) {
		return fmt.Errorf("validation error: some credentials are empty")
	}

	return nil
}

func anyIsEmpty(ss ...string) bool {
	zero := false
	for _, s := range ss {
		zero = zero || len(s) == 0
	}
	return zero
}

// nolint: unparam
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

func shouldUpdateToVersion(cluster *mysqlcluster.MysqlCluster, targetVersion int) bool {
	var version string
	var ok bool
	if version, ok = cluster.ObjectMeta.Annotations["mysql.presslabs.org/version"]; !ok {
		// no version annotation present, (it's a cluster older than 0.3.0) or it's a new cluster
		log.Info("annotation not set on cluster")
		return true
	}

	ver, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		log.Error(err, "annotation version can't be parsed", "value", version)
		return true
	}

	return int(ver) < targetVersion
}
