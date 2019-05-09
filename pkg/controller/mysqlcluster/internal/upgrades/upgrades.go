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

package upgrades

import (
	"context"
	"fmt"
	"strconv"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

var log = logf.Log.WithName("upgrades.cluster")

const (
	// VersionAnnotation represents the annotation used to annotate a cluster to it's version
	VersionAnnotation = "mysql.presslabs.org/version"
)

// Interface represents the upgrader interface
type Interface interface {
	ShouldUpdate() bool
	Run(context.Context) error
}

type upgrader struct {
	version int
	cluster *mysqlcluster.MysqlCluster

	recorder  record.EventRecorder
	client    client.Client
	orcClient orc.Interface
}

// nolint: gocyclo
func (u *upgrader) Run(ctx context.Context) error {

	sts, err := u.getStatefulSet(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			// no stateful set found
			return u.markUpgradeComplete()
		}
		return err
	}

	insts, err := u.instancesFromOrc()
	if err != nil {
		return err
	}

	// more than 1 replica so there is the case when node 0 is slave so mark all other nodes as
	// in maintenance except node 0.
	// TODO: or set promotion rules
	if int(*sts.Spec.Replicas) > 1 {
		one := int32(1)
		sts.Spec.Replicas = &one
		if err = u.client.Update(ctx, sts); err != nil {
			return err
		}

		// set ready nodes on cluster to 0
		u.cluster.Status.ReadyNodes = 0
		if err = u.client.Status().Update(ctx, u.cluster.Unwrap()); err != nil {
			return err
		}
	}

	if sts.Status.ReadyReplicas > 1 {
		return fmt.Errorf("statefulset has more than one running pods")
	}

	if err = u.checkNode0Ok(insts); err != nil {
		return err
	}

	// forget all nodes from orchestrator, no need for them there, when sts will be recreated the
	// nodes will be inaccessible.
	if err = u.forgetFromOrc(); err != nil {
		return err
	}

	// delete stateful set
	if err = u.client.Delete(ctx, sts); err != nil {
		return err
	}

	// delete the old headless service
	hlSVC := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      u.cluster.GetNameForResource(mysqlcluster.OldHeadlessSVC),
			Namespace: u.cluster.Namespace,
		},
	}
	if err = u.client.Delete(ctx, hlSVC); err != nil {
		return err
	}

	return u.markUpgradeComplete()
}

func (u *upgrader) ShouldUpdate() bool {
	var version string
	var ok bool
	if version, ok = u.cluster.ObjectMeta.Annotations[VersionAnnotation]; !ok {
		// no version annotation present, (it's a cluster older than 0.3.0) or it's a new cluster
		log.Info("annotation not set on cluster")
		return true
	}

	ver, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		log.Error(err, "annotation version can't be parsed", "value", version)
		return true
	}

	return int(ver) < u.version
}

func (u *upgrader) markUpgradeComplete() error {
	if u.cluster.Annotations == nil {
		u.cluster.Annotations = make(map[string]string)
	}
	u.cluster.Annotations[VersionAnnotation] = strconv.Itoa(u.version)
	err := u.client.Update(context.TODO(), u.cluster.Unwrap())
	if err != nil {
		log.Error(err, "failed to update cluster spec", "cluster", u.cluster)
		return err
	}
	return err
}

func (u *upgrader) getStatefulSet(ctx context.Context) (*apps.StatefulSet, error) {
	sts := &apps.StatefulSet{}
	stsKey := types.NamespacedName{
		Name:      u.cluster.GetNameForResource(mysqlcluster.StatefulSet),
		Namespace: u.cluster.Namespace,
	}
	return sts, u.client.Get(ctx, stsKey, sts)
}

func (u *upgrader) instancesFromOrc() ([]orc.Instance, error) {
	var insts []orc.Instance
	var err error
	if insts, err = u.orcClient.Cluster(u.cluster.GetClusterAlias()); err != nil {
		if !orc.IsNotFound(err) {
			return nil, err
		}
		// not found in orchestrator, continue
		return insts, nil
	}
	return insts, nil
}

func (u *upgrader) checkNode0Ok(insts []orc.Instance) error {
	node0 := u.getNodeFrom(insts, 0)
	if node0 == nil {
		// continue
		log.Info("no node found in orchestraotr")
		return fmt.Errorf("node-0 not found in orchestarotr")
	}

	if len(node0.MasterKey.Hostname) != 0 {
		if !node0.IsDetachedMaster {
			// node 0 is not master
			return fmt.Errorf("node 0 not yet master")
		}
	}

	lag := node0.SecondsBehindMaster
	if lag.Valid && lag.Int64 > int64(3) {
		return fmt.Errorf("node 0 is lagged")
	}

	return nil
}

func (u *upgrader) getNodeFrom(insts []orc.Instance, node int) *orc.Instance {
	for _, inst := range insts {
		if inst.Key.Hostname == u.getPodOldHostname(node) {
			return &inst
		}
	}
	return nil

}

func (u *upgrader) getPodOldHostname(node int) string {
	return fmt.Sprintf("%s-%d.%s.%s", u.cluster.GetNameForResource(mysqlcluster.StatefulSet), node,
		u.cluster.GetNameForResource(mysqlcluster.OldHeadlessSVC),
		u.cluster.Namespace)
}

func (u *upgrader) forgetFromOrc() error {
	for node := 0; node < int(*u.cluster.Spec.Replicas); node++ {
		if err := u.orcClient.Forget(u.getPodOldHostname(node), 3306); err != nil {
			return err
		}
	}

	return nil
}

// NewUpgrader returns a upgrader
func NewUpgrader(client client.Client, recorder record.EventRecorder, cluster *mysqlcluster.MysqlCluster, opt *options.Options) Interface {

	return &upgrader{
		cluster:   cluster,
		recorder:  recorder,
		client:    client,
		orcClient: orc.NewFromURI(opt.OrchestratorURI),
		version:   300, // TODO
	}
}
