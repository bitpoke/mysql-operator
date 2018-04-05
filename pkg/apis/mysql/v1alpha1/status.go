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

package v1alpha1

import (
	"time"

	"github.com/golang/glog"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (c *MysqlCluster) UpdateStatusCondition(condType ClusterConditionType,
	status apiv1.ConditionStatus, reason, msg string) {
	newCondition := ClusterCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(c.Status.Conditions) == 0 {
		glog.Infof("Setting lastTransitionTime for mysql cluster "+
			"%q condition %q to %v", c.Name, condType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []ClusterCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[0]
			if cond.Status != newCondition.Status {
				glog.Infof("Found status change for mysql cluster "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t)
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			glog.Infof("Setting lastTransitionTime for mysql cluster "+
				"%q condition %q to %q", c.Name, condType, status)
			c.Status.Conditions[i] = newCondition
		} else {
			glog.Infof("Setting new condition for mysql cluster %q, condition %q to %q",
				c.Name, condType, status)
			newCondition.LastTransitionTime = metav1.NewTime(t)
			c.Status.Conditions = append(c.Status.Conditions, newCondition)
		}
	}
}

func (c *MysqlCluster) condExists(ty ClusterConditionType) (int, bool) {
	for i, cond := range c.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

func (b *MysqlBackup) GetCondition(ty BackupConditionType) *BackupCondition {
	for _, cond := range b.Status.Conditions {
		if cond.Type == ty {
			return &cond
		}
	}

	return nil
}

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (c *MysqlBackup) UpdateStatusCondition(condType BackupConditionType,
	status apiv1.ConditionStatus, reason, msg string) {
	newCondition := BackupCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(c.Status.Conditions) == 0 {
		glog.Infof("Setting lastTransitionTime for mysql backup "+
			"%q condition %q to %v", c.Name, condType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []BackupCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[0]
			if cond.Status != newCondition.Status {
				glog.Infof("Found status change for mysql backup "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t)
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			glog.Infof("Setting lastTransitionTime for mysql backup "+
				"%q condition %q to %q", c.Name, condType, status)
			c.Status.Conditions[i] = newCondition
		} else {
			glog.Infof("Setting new condition for mysql backup %q, condition %q to %q",
				c.Name, condType, status)
			newCondition.LastTransitionTime = metav1.NewTime(t)
			c.Status.Conditions = append(c.Status.Conditions, newCondition)
		}
	}
}

func (c *MysqlBackup) condExists(ty BackupConditionType) (int, bool) {
	for i, cond := range c.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

// Mysql events reason
const (
	EventReasonInitDefaults         = "InitDefaults"
	EventReasonInitDefaultsFailed   = "InitDefaultsFailed"
	EventReasonDbSecretUpdated      = "DbSecretUpdated"
	EventReasonDbSecretFailed       = "DbSecretFailed"
	EventReasonUtilitySecretFailed  = "UtilitySecretFailed"
	EventReasonUtilitySecretUpdated = "UtilitySecretUpdated"
	EventReasonConfigMapFailed      = "MysqlConfigMapFailed"
	EventReasonConfigMapUpdated     = "MysqlConfigMapUpdated"
	EventReasonServiceFailed        = "HLServiceFailed"
	EventReasonServiceUpdated       = "HLServiceUpdated"
	EventReasonSFSFailed            = "StatefulSetFailed"
	EventReasonSFSUpdated           = "StatefulSetUpdated"
	EventReasonCronJobFailed        = "CronJobFailed"
	EventReasonCronJobUpdated       = "CronJobUpdated"
)

// Event types
const (
	EventNormal  = "Normal"
	EventWarning = "Warning"
)
