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

// nolint: errcheck,golint
package testutil

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// NodeConditions returns a list of api.NodeConditions for a node
func NodeConditions(master, replicating, lagged, readOnly bool) []api.NodeCondition {
	masterCond := core.ConditionFalse
	if master {
		masterCond = core.ConditionTrue
	}
	replicatingCond := core.ConditionFalse
	if replicating {
		replicatingCond = core.ConditionTrue
	}
	laggedCond := core.ConditionFalse
	if lagged {
		laggedCond = core.ConditionTrue
	}
	readOnlyCond := core.ConditionFalse
	if readOnly {
		readOnlyCond = core.ConditionTrue
	}

	t := metav1.NewTime(time.Now())
	return []api.NodeCondition{
		{
			Type:               api.NodeConditionMaster,
			Status:             masterCond,
			LastTransitionTime: t,
		},
		{
			Type:               api.NodeConditionReplicating,
			Status:             replicatingCond,
			LastTransitionTime: t,
		},
		{
			Type:               api.NodeConditionLagged,
			Status:             laggedCond,
			LastTransitionTime: t,
		},
		{
			Type:               api.NodeConditionReadOnly,
			Status:             readOnlyCond,
			LastTransitionTime: t,
		},
	}
}

// RefreshFn receives a client and a runtime.Objects and refreshes the object from k8s
// example: Eventually(RefreshFn(c, cluster.Unwrap())).Should(HaveClusterStatusReadyNodes(2))
func RefreshFn(c client.Client, obj runtime.Object) func() runtime.Object {
	return func() runtime.Object {
		objMeta, ok := obj.(metav1.Object)
		if !ok {
			return nil
		}

		objKey := types.NamespacedName{
			Name:      objMeta.GetName(),
			Namespace: objMeta.GetNamespace(),
		}

		if err := c.Get(context.TODO(), objKey, obj); err == nil {
			return obj
		}

		// if the object is not updated then return nil, not the old object
		return nil
	}
}

// HaveClusterStatusReadyNodes a matcher that checks cluster ready nodes to equal the given value
func HaveClusterStatusReadyNodes(nodes int) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"ReadyNodes": Equal(nodes),
		}),
	}))
}
