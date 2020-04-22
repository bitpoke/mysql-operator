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

package mysqldatabase

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const (
	// ProvisionSucceeded is used as the reason for the condition
	ProvisionSucceeded = "ProvisionSucceeded"

	// ProvisionFailed the reason when creation fails
	ProvisionFailed = "ProvisionFailed"
)

// Database is a wrapper over MySQLDatabase k8s resource
type Database struct {
	*mysqlv1alpha1.MySQLDatabase
}

// Wrap wraps a MySQLDatabase
func Wrap(db *mysqlv1alpha1.MySQLDatabase) *Database {
	return &Database{
		MySQLDatabase: db,
	}
}

// Unwrap returns the MySQLDatabase object
func (db *Database) Unwrap() *mysqlv1alpha1.MySQLDatabase {
	return db.MySQLDatabase
}

// ConditionExists returns a condition and whether it exists
func (db *Database) ConditionExists(
	ct mysqlv1alpha1.MySQLDatabaseConditionType,
) (
	*mysqlv1alpha1.MySQLDatabaseCondition, bool,
) {
	for i := range db.Status.Conditions {
		cond := &db.Status.Conditions[i]
		if cond.Type == ct {
			return cond, true
		}
	}

	return nil, false
}

// UpdateCondition updates the site's condition matching the given type
func (db *Database) UpdateCondition(
	condType mysqlv1alpha1.MySQLDatabaseConditionType, status corev1.ConditionStatus, reason, message string,
) (
	cond *mysqlv1alpha1.MySQLDatabaseCondition, changed bool,
) {
	t := metav1.NewTime(time.Now())

	existingCondition, exists := db.ConditionExists(condType)
	if !exists {
		newCondition := mysqlv1alpha1.MySQLDatabaseCondition{
			Type:               condType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: t,
			LastUpdateTime:     t,
		}
		db.Status.Conditions = append(db.Status.Conditions, newCondition)

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

// GetClusterKey is a helper function that returns the mysql cluster object key
func (db *Database) GetClusterKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      db.Spec.ClusterRef.Name,
		Namespace: db.Spec.ClusterRef.Namespace,
	}
}
