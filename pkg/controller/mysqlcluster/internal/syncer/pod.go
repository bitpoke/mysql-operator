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
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-test/deep"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

type podSyncer struct {
	cluster  *mysqlcluster.MysqlCluster
	hostname string
	log      logr.Logger
}

const (
	labelMaster    = "master"
	labelReplica   = "replica"
	labelHealty    = "yes"
	labelNotHealty = "no"
)

// NewPodSyncer returns the syncer for pod
func NewPodSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, host string) syncer.Interface {
	pod := &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPodNameForHost(host),
			Namespace: cluster.Namespace,
		},
	}

	sync := &podSyncer{
		cluster:  cluster,
		hostname: host,
		log:      logf.Log.WithName("pod-syncer").WithValues("key", cluster),
	}

	return syncer.NewObjectSyncer("Pod", nil, pod, c, scheme, func() error {
		return sync.SyncFn(pod)
	})
}

// nolint: gocyclo
func (s *podSyncer) SyncFn(out *core.Pod) error {
	// raise error if pod is not created
	if out.CreationTimestamp.IsZero() {
		return NewPodNotFoundError()
	}

	master := s.cluster.GetNodeCondition(s.hostname, api.NodeConditionMaster)
	replicating := s.cluster.GetNodeCondition(s.hostname, api.NodeConditionReplicating)
	lagged := s.cluster.GetNodeCondition(s.hostname, api.NodeConditionLagged)

	if master == nil {
		return fmt.Errorf("master status condition not set")
	}

	isMaster := master.Status == core.ConditionTrue
	isLagged := lagged != nil && lagged.Status == core.ConditionTrue
	isReplicating := replicating != nil && replicating.Status == core.ConditionTrue

	// set role label
	role := labelReplica
	if isMaster {
		role = labelMaster
	}

	// set healthy label
	healthy := labelNotHealty
	if isMaster || !isMaster && isReplicating && !isLagged {
		healthy = labelHealty
	}

	oldLabels := out.ObjectMeta.Labels

	if len(out.ObjectMeta.Labels) == 0 {
		out.ObjectMeta.Labels = map[string]string{}
	}

	out.ObjectMeta.Labels["role"] = role
	out.ObjectMeta.Labels["healthy"] = healthy

	if !reflect.DeepEqual(oldLabels, out.ObjectMeta.Labels) {
		s.log.Info("node labels updated", "host", out.Spec.Hostname, "diff", deep.Equal(oldLabels, out.ObjectMeta.Labels))
	}

	return nil
}

func getPodNameForHost(host string) string {
	return strings.SplitN(host, ".", 2)[0]
}
