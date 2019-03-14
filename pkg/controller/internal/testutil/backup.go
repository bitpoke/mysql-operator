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

// nolint: golint, errcheck
package testutil

import (
	"context"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// ListAllBackupsFn returns a helper function that can be used with gomega
// Eventually and Consistently
func ListAllBackupsFn(c client.Client, options *client.ListOptions) func() []api.MysqlBackup {
	return func() []api.MysqlBackup {
		backups := &api.MysqlBackupList{}
		Expect(c.List(context.TODO(), options, backups)).To(Succeed())
		return backups.Items
	}
}

// BackupHaveCondition is a helper func that returns a matcher to check for an
// existing condition in condition list list
func BackupHaveCondition(condType api.BackupConditionType, status core.ConditionStatus) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(condType),
				"Status": Equal(status),
			})),
		}),
	}))
}

// BackupForCluster is gomega matcher that matches a backup which is for given
// cluster
func BackupForCluster(cluster *api.MysqlCluster) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Spec": MatchFields(IgnoreExtras, Fields{
			"ClusterName": Equal(cluster.Name),
		}),
	})
}

// BackupWithName is a gomega matcher that matchers a backup with the given name
func BackupWithName(name string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"ClusterName": Equal(name),
		}),
	})
}
