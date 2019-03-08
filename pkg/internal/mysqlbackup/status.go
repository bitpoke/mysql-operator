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

package mysqlbackup

import (
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (c *MysqlBackup) UpdateStatusCondition(condType api.BackupConditionType,
	status core.ConditionStatus, reason, msg string) {
	newCondition := api.BackupCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(c.Status.Conditions) == 0 {
		log.V(4).Info(fmt.Sprintf("Setting lastTransitionTime for mysql backup "+
			"%q condition %q to %v", c.Name, condType, t))
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []api.BackupCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				log.V(3).Info(fmt.Sprintf("Found status change for mysql backup "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t))
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			log.V(4).Info(fmt.Sprintf("Setting lastTransitionTime for mysql backup "+
				"%q condition %q to %q", c.Name, condType, status))
			c.Status.Conditions[i] = newCondition
		} else {
			log.V(4).Info(fmt.Sprintf("Setting new condition for mysql backup %q, condition %q to %q",
				c.Name, condType, status))
			newCondition.LastTransitionTime = metav1.NewTime(t)
			c.Status.Conditions = append(c.Status.Conditions, newCondition)
		}
	}
}

func (c *MysqlBackup) condExists(ty api.BackupConditionType) (int, bool) {
	for i, cond := range c.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

// GetBackupCondition returns a pointer to the condition of the provided type
func (c *MysqlBackup) GetBackupCondition(condType api.BackupConditionType) *api.BackupCondition {
	i, found := c.condExists(condType)
	if found {
		return &c.Status.Conditions[i]
	}

	return nil
}
