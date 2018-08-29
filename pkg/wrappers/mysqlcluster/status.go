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
	"time"

	"github.com/golang/glog"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// UpdateStatusCondition sets the condition to a status.
// for example Ready condition to True, or False
func (c *MysqlCluster) UpdateStatusCondition(condType api.ClusterConditionType,
	status core.ConditionStatus, reason, msg string) {
	newCondition := api.ClusterCondition{
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
		c.Status.Conditions = []api.ClusterCondition{newCondition}
	} else {
		if i, exist := c.condExists(condType); exist {
			cond := c.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				glog.V(3).Infof("Found status change for mysql cluster "+
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

func (c *MysqlCluster) condExists(ty api.ClusterConditionType) (int, bool) {
	for i, cond := range c.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

func (c *MysqlCluster) UpdateConditionForNode(name string, cType api.NodeConditionType, cStatus core.ConditionStatus) bool {
	i := c.GetNodeStatusIndex(name)
	return updateNodeCondition(&c.Status.Nodes[i], cType, cStatus)
}

// UpdateNodeCondition updates the condition for a given type
func updateNodeCondition(ns *api.NodeStatus, cType api.NodeConditionType,
	cStatus core.ConditionStatus) bool {

	newCondition := api.NodeCondition{
		Type:   cType,
		Status: cStatus,
	}

	changed := false
	t := time.Now()

	if len(ns.Conditions) == 0 {
		glog.V(4).Infof("Setting lastTransitionTime for node "+
			"%q condition %q to %v", ns.Name, cType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		ns.Conditions = []api.NodeCondition{newCondition}
		changed = true
	} else {
		if i, exist := nsCondExists(ns, cType); exist {
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

func nsCondExists(ns *api.NodeStatus, cType api.NodeConditionType) (int, bool) {
	for i, cond := range ns.Conditions {
		if cond.Type == cType {
			return i, true
		}
	}

	return 0, false
}

func (c *MysqlCluster) GetNodeStatusIndex(name string) int {
	for i := 0; i < len(c.Status.Nodes); i++ {
		if c.Status.Nodes[i].Name == name {
			return i
		}
	}

	c.Status.Nodes = append(c.Status.Nodes, api.NodeStatus{Name: name})
	return c.GetNodeStatusIndex(name)

}

func (c *MysqlCluster) GetNodeCondition(name string, cType api.NodeConditionType) *api.NodeCondition {
	nodeStatusIndex := c.GetNodeStatusIndex(name)
	condIndex, exists := nsCondExists(&c.Status.Nodes[nodeStatusIndex], cType)
	if exists {
		return &c.Status.Nodes[nodeStatusIndex].Conditions[condIndex]
	}

	return nil
}
