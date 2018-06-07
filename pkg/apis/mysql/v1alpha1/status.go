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

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (c *MysqlCluster) UpdateStatusCondition(condType ClusterConditionType,
	status core.ConditionStatus, reason, msg string) {
	newCondition := ClusterCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(c.Status.Conditions) == 0 {
		glog.V(4).Infof("Setting lastTransitionTime for mysql cluster "+
			"%q condition %q to %v", c.Name, condType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []ClusterCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				glog.V(4).Infof("Found status change for mysql cluster "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t)
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			glog.V(4).Infof("Setting lastTransitionTime for mysql cluster "+
				"%q condition %q to %q", c.Name, condType, status)
			c.Status.Conditions[i] = newCondition
		} else {
			glog.V(4).Infof("Setting new condition for mysql cluster %q, condition %q to %q",
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
	status core.ConditionStatus, reason, msg string) {
	newCondition := BackupCondition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

	t := time.Now()

	if len(c.Status.Conditions) == 0 {
		glog.V(4).Infof("Setting lastTransitionTime for mysql backup "+
			"%q condition %q to %v", c.Name, condType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []BackupCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				glog.V(4).Infof("Found status change for mysql backup "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t)
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			glog.V(4).Infof("Setting lastTransitionTime for mysql backup "+
				"%q condition %q to %q", c.Name, condType, status)
			c.Status.Conditions[i] = newCondition
		} else {
			glog.V(4).Infof("Setting new condition for mysql backup %q, condition %q to %q",
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
	EventReasonInitDefaults               = "InitDefaults"
	EventReasonInitDefaultsFailed         = "InitDefaultsFailed"
	EventReasonDbSecretUpdated            = "DbSecretUpdated"
	EventReasonDbSecretFailed             = "DbSecretFailed"
	EventReasonUtilitySecretFailed        = "UtilitySecretFailed"
	EventReasonUtilitySecretUpdated       = "UtilitySecretUpdated"
	EventReasonConfigMapFailed            = "MysqlConfigMapFailed"
	EventReasonConfigMapUpdated           = "MysqlConfigMapUpdated"
	EventReasonServiceFailed              = "HLServiceFailed"
	EventReasonServiceUpdated             = "HLServiceUpdated"
	EventReasonSFSFailed                  = "StatefulSetFailed"
	EventReasonSFSUpdated                 = "StatefulSetUpdated"
	EventReasonMasterServiceFailed        = "MasterServiceFailed"
	EventReasonMasterServiceUpdated       = "MasterServiceUpdated"
	EventReasonHealthyNodesServiceFailed  = "HealthyNodesServiceFailed"
	EventReasonHealthyNodesServiceUpdated = "HealthyNodesServiceUpdated"
)

// Event types
const (
	EventNormal  = "Normal"
	EventWarning = "Warning"
)

func (ns *NodeStatus) UpdateNodeCondition(cType NodeConditionType,
	cStatus core.ConditionStatus) bool {

	newCondition := NodeCondition{
		Type:   cType,
		Status: cStatus,
	}

	changed := false
	t := time.Now()

	if len(ns.Conditions) == 0 {
		glog.V(4).Infof("Setting lastTransitionTime for node "+
			"%q condition %q to %v", ns.Name, cType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		ns.Conditions = []NodeCondition{newCondition}
		changed = true
	} else {
		if i, exist := ns.condExists(cType); exist {
			cond := ns.Conditions[i]
			if cond.Status != newCondition.Status {
				glog.V(4).Infof("Found status change for node "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					ns.Name, cType, cond.Status, cStatus, t)
				newCondition.LastTransitionTime = metav1.NewTime(t)
				changed = true
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			glog.V(4).Infof("Setting lastTransitionTime for node "+
				"%q condition %q to %q", ns.Name, cType, cStatus)
			ns.Conditions[i] = newCondition
		} else {
			glog.V(4).Infof("Setting new condition for node %q, condition %q to %q",
				ns.Name, cType, cStatus)
			newCondition.LastTransitionTime = metav1.NewTime(t)
			ns.Conditions = append(ns.Conditions, newCondition)
			changed = true
		}
	}

	return changed
}

func (ns *NodeStatus) condExists(cType NodeConditionType) (int, bool) {
	for i, cond := range ns.Conditions {
		if cond.Type == cType {
			return i, true
		}
	}

	return 0, false
}

func (s *ClusterStatus) GetNodeStatusIndex(name string) int {
	for i := 0; i < len(s.Nodes); i++ {
		if s.Nodes[i].Name == name {
			return i
		}
	}
	s.Nodes = append(s.Nodes, NodeStatus{Name: name})
	return s.GetNodeStatusIndex(name)
}

func (ns *NodeStatus) GetCondition(cType NodeConditionType) *NodeCondition {
	if i, exist := ns.condExists(cType); exist {
		return &ns.Conditions[i]
	}

	return nil
}
