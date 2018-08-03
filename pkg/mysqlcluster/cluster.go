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

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myclientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	"github.com/presslabs/mysql-operator/pkg/util/options"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

// Interface is for cluster Factory
type Interface interface {
	// Sync is the method that tries to sync the cluster.
	Sync(ctx context.Context) error

	// SyncOrchestratorStatus brings info from orchestrator on resource status
	SyncOrchestratorStatus(ctx context.Context) error

	// Reconcile handle some action that need to be done recurrent over a cluster
	Reconcile(ctx context.Context) error
}

// cluster factory
type cFactory struct {
	cluster *api.MysqlCluster
	opt     *options.Options

	namespace string

	client   kubernetes.Interface
	myClient myclientset.Interface
	rec      record.EventRecorder

	orcClient orc.Interface

	configHash string
	secretHash string
}

// New creates a new cluster factory
func New(cluster *api.MysqlCluster, opt *options.Options, klient kubernetes.Interface,
	myClient myclientset.Interface, ns string, rec record.EventRecorder) Interface {
	f := &cFactory{
		cluster:    cluster,
		opt:        opt,
		client:     klient,
		myClient:   myClient,
		namespace:  ns,
		rec:        rec,
		configHash: "1",
		secretHash: "1",
	}
	if len(cluster.Spec.GetOrcUri()) != 0 {
		f.orcClient = orc.NewFromUri(cluster.Spec.GetOrcUri())
	}

	return f
}

const (
	statusUpToDate = "up-to-date"
	statusCreated  = "created"
	statusUpdated  = "updated"
	statusFailed   = "failed"
	statusOk       = "ok"
	statusSkip     = "skip"
	statusDeleted  = "deleted"
)

type component struct {
	// the name that will be showed in logs
	alias  string
	name   string
	syncFn func() (string, error)
	//event reason when sync faild
	reasonFailed string
	// event reason when object is updated
	reasonUpdated string
}

func (f *cFactory) getComponents() []component {
	return []component{
		component{
			alias:         "cluster-secret",
			name:          f.cluster.Spec.SecretName,
			syncFn:        f.syncClusterSecret,
			reasonFailed:  api.EventReasonDbSecretFailed,
			reasonUpdated: api.EventReasonDbSecretUpdated,
		},
		component{
			alias:         "config-map",
			name:          f.cluster.GetNameForResource(api.ConfigMap),
			syncFn:        f.syncConfigMysqlMap,
			reasonFailed:  api.EventReasonConfigMapFailed,
			reasonUpdated: api.EventReasonConfigMapUpdated,
		},
		component{
			alias:         "headless-service",
			name:          f.cluster.GetNameForResource(api.HeadlessSVC),
			syncFn:        f.syncHeadlessService,
			reasonFailed:  api.EventReasonServiceFailed,
			reasonUpdated: api.EventReasonServiceUpdated,
		},
		component{
			alias:         "statefulset",
			name:          f.cluster.GetNameForResource(api.StatefulSet),
			syncFn:        f.syncStatefulSet,
			reasonFailed:  api.EventReasonSFSFailed,
			reasonUpdated: api.EventReasonSFSUpdated,
		},
		component{
			alias:         "master-service",
			name:          f.cluster.GetNameForResource(api.MasterService),
			syncFn:        f.syncMasterService,
			reasonFailed:  api.EventReasonMasterServiceFailed,
			reasonUpdated: api.EventReasonMasterServiceUpdated,
		},
		component{
			alias:         "healthy-service",
			name:          f.cluster.GetNameForResource(api.HealthyNodesService),
			syncFn:        f.syncHealthyNodesService,
			reasonFailed:  api.EventReasonHealthyNodesServiceFailed,
			reasonUpdated: api.EventReasonHealthyNodesServiceUpdated,
		},
		component{
			alias:         "pdb",
			name:          f.cluster.GetNameForResource(api.PodDisruptionBudget),
			syncFn:        f.syncPDB,
			reasonFailed:  api.EventReasonPDBFailed,
			reasonUpdated: api.EventReasonPDBUpdated,
		},
	}
}

func (f *cFactory) Sync(ctx context.Context) error {
	for _, comp := range f.getComponents() {
		state, err := comp.syncFn()
		if err != nil {
			glog.Warningf("[%s]: failed syncing %s: ", comp.alias, comp.name, err.Error())
			err = fmt.Errorf("%s sync failed: %s", comp.name, err)
			f.rec.Event(f.cluster, api.EventWarning, comp.reasonFailed, err.Error())
			return err
		} else {
			glog.V(2).Infof("[%s]: %s ... (%s)", comp.alias, comp.name, state)
		}
		switch state {
		case statusCreated, statusUpdated:
			f.rec.Event(f.cluster, api.EventNormal, comp.reasonUpdated, "")
		}
	}

	// update master endpoints
	if err := f.updateMasterServiceEndpoints(); err != nil {
		return fmt.Errorf("cluster sync master endpoints: %s", err)
	}

	if err := f.updateHealthyNodesServiceEndpoints(); err != nil {
		return fmt.Errorf("cluster sync ready endpoints: %s", err)
	}

	if err := f.updatePodLabels(); err != nil {
		return fmt.Errorf("cluster sync pod labeling: %s", err)
	}
	return nil
}

func (f *cFactory) Reconcile(ctx context.Context) error {
	// Update healty nodes endpoints
	return f.updateHealthyNodesServiceEndpoints()
}

func (f *cFactory) getOwnerReferences(ors ...[]metav1.OwnerReference) []metav1.OwnerReference {
	rs := []metav1.OwnerReference{
		f.cluster.AsOwnerReference(),
	}
	for _, or := range ors {
		for _, o := range or {
			rs = append(rs, o)
		}
	}
	return rs
}

func (f *cFactory) getClusterAlias() string {
	return fmt.Sprintf("%s.%s", f.cluster.Name, f.cluster.Namespace)
}
