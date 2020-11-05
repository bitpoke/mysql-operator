/*
Copyright 2020 Pressinfra SRL

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

package mysqluser

import (
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const (
	// ProvisionFailedReason is the condition reason when MysqlUser provisioning
	// has failed.
	ProvisionFailedReason = "ProvisionFailed"
	// ProvisionInProgressReason is the reason when MysqlUser provisioning has
	// started.
	ProvisionInProgressReason = "ProvisionInProgress"

	// ProvisionSucceededReason the reason used when provision was successful.
	ProvisionSucceededReason = "ProvisionSucceeded"
)

// MySQLUser embeds mysqlv1alpha1.MysqlUser and adds utility functions
type MySQLUser struct {
	*mysqlv1alpha1.MysqlUser
}

// Wrap wraps a mysqlv1alpha1.AccountBinding into an AccountBinding object
func Wrap(u *mysqlv1alpha1.MysqlUser) *MySQLUser {
	return &MySQLUser{u}
}

// Unwrap returns the wrapped AccountBinding object
func (u *MySQLUser) Unwrap() *mysqlv1alpha1.MysqlUser {
	return u.MysqlUser
}

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (u *MySQLUser) UpdateStatusCondition(
	condType mysqlv1alpha1.MysqlUserConditionType,
	status v1.ConditionStatus, reason, message string,
) (
	cond *mysqlv1alpha1.MySQLUserCondition, changed bool,
) {
	t := metav1.NewTime(time.Now())

	existingCondition, exists := u.ConditionExists(condType)
	if !exists {
		newCondition := mysqlv1alpha1.MySQLUserCondition{
			Type:               condType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: t,
			LastUpdateTime:     t,
		}
		u.Status.Conditions = append(u.Status.Conditions, newCondition)

		return &newCondition, true
	}

	if status != existingCondition.Status {
		existingCondition.LastTransitionTime = t
		changed = true
	}

	if message != existingCondition.Message || reason != existingCondition.Reason {
		existingCondition.LastUpdateTime = t
		changed = true
	}

	existingCondition.Status = status
	existingCondition.Message = message
	existingCondition.Reason = reason

	return existingCondition, changed
}

// ConditionExists returns a condition and whether it exists
func (u *MySQLUser) ConditionExists(
	ct mysqlv1alpha1.MysqlUserConditionType,
) (
	*mysqlv1alpha1.MySQLUserCondition, bool,
) {
	for i := range u.Status.Conditions {
		cond := &u.Status.Conditions[i]
		if cond.Type == ct {
			return cond, true
		}
	}

	return nil, false
}

// GetClusterKey returns the MysqlUser's MySQLCluster key
func (u *MySQLUser) GetClusterKey() client.ObjectKey {
	ns := u.Spec.ClusterRef.Namespace
	if ns == "" {
		ns = u.Namespace
	}

	return client.ObjectKey{
		Name:      u.Spec.ClusterRef.Name,
		Namespace: ns,
	}
}

// GetKey return the user key. Usually used for logging or for runtime.Client.Get as key
func (u *MySQLUser) GetKey() client.ObjectKey {
	return types.NamespacedName{
		Namespace: u.Namespace,
		Name:      u.Name,
	}
}
