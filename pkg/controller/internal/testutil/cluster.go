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

package testutil

import (
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		api.NodeCondition{
			Type:               api.NodeConditionMaster,
			Status:             masterCond,
			LastTransitionTime: t,
		},
		api.NodeCondition{
			Type:               api.NodeConditionReplicating,
			Status:             replicatingCond,
			LastTransitionTime: t,
		},
		api.NodeCondition{
			Type:               api.NodeConditionLagged,
			Status:             laggedCond,
			LastTransitionTime: t,
		},
		api.NodeCondition{
			Type:               api.NodeConditionReadOnly,
			Status:             readOnlyCond,
			LastTransitionTime: t,
		},
	}
}
