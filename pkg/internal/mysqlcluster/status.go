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
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var log = logf.Log.WithName("update-status")

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
		log.V(4).Info(fmt.Sprintf("Setting lastTransitionTime for mysql cluster "+
			"%q condition %q to %v", c.Name, condType, t))
		newCondition.LastTransitionTime = metav1.NewTime(t)
		c.Status.Conditions = []api.ClusterCondition{newCondition}
	} else {
		if i, exist := c.condIndex(condType); exist {
			cond := c.Status.Conditions[i]
			if cond.Status != newCondition.Status {
				log.V(3).Info(fmt.Sprintf("Found status change for mysql cluster "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					c.Name, condType, cond.Status, status, t))
				newCondition.LastTransitionTime = metav1.NewTime(t)
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			log.V(4).Info(fmt.Sprintf("Setting lastTransitionTime for mysql cluster "+
				"%q condition %q to %q", c.Name, condType, status))
			c.Status.Conditions[i] = newCondition
		} else {
			log.V(4).Info(fmt.Sprintf("Setting new condition for mysql cluster %q, condition %q to %q",
				c.Name, condType, status))
			newCondition.LastTransitionTime = metav1.NewTime(t)
			c.Status.Conditions = append(c.Status.Conditions, newCondition)
		}
	}
}

func (c *MysqlCluster) condIndex(condType api.ClusterConditionType) (int, bool) {
	for i, cond := range c.Status.Conditions {
		if cond.Type == condType {
			return i, true
		}
	}

	return 0, false
}

// GetClusterCondition returns the cluster condition of the given type
func (c *MysqlCluster) GetClusterCondition(condType api.ClusterConditionType) *api.ClusterCondition {
	i, found := c.condIndex(condType)
	if found {
		return &c.Status.Conditions[i]
	}

	return nil
}

// UpdateNodeConditionStatus updates the status of the condition for a given name and type
func (c *MysqlCluster) UpdateNodeConditionStatus(nodeName string, condType api.NodeConditionType, status core.ConditionStatus) bool {
	i := c.GetNodeStatusIndex(nodeName)
	return updateNodeCondition(&c.Status.Nodes[i], condType, status)
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
		log.V(3).Info(fmt.Sprintf("Setting lastTransitionTime for node "+
			"%q condition %q to %v", ns.Name, cType, t))
		newCondition.LastTransitionTime = metav1.NewTime(t)
		ns.Conditions = []api.NodeCondition{newCondition}
		changed = true
	} else {
		if i, exist := GetNodeConditionIndex(ns, cType); exist {
			cond := ns.Conditions[i]
			if cond.Status != newCondition.Status {
				log.V(3).Info(fmt.Sprintf("Found status change for node "+
					"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
					ns.Name, cType, cond.Status, cStatus, t))
				newCondition.LastTransitionTime = metav1.NewTime(t)
				changed = true
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			log.V(3).Info(fmt.Sprintf("Setting lastTransitionTime for node "+
				"%q condition %q to %q", ns.Name, cType, cStatus))
			ns.Conditions[i] = newCondition
		} else {
			log.V(3).Info(fmt.Sprintf("Setting new condition for node %q, condition %q to %q",
				ns.Name, cType, cStatus))
			newCondition.LastTransitionTime = metav1.NewTime(t)
			ns.Conditions = append(ns.Conditions, newCondition)
			changed = true
		}
	}

	return changed
}

// GetNodeConditionIndex returns the index of a condition. The boolean value is
// true if the conditions exists otherwise is false.
func GetNodeConditionIndex(nodeStatus *api.NodeStatus, condType api.NodeConditionType) (int, bool) {
	for i, cond := range nodeStatus.Conditions {
		if cond.Type == condType {
			return i, true
		}
	}

	return 0, false
}

// GetNodeStatusIndex get index of node given the name
func (c *MysqlCluster) GetNodeStatusIndex(name string) int {
	for i := 0; i < len(c.Status.Nodes); i++ {
		if c.Status.Nodes[i].Name == name {
			return i
		}
	}

	c.Status.Nodes = append(c.Status.Nodes, api.NodeStatus{Name: name})
	return c.GetNodeStatusIndex(name)

}

// GetNodeCondition get NodeCondigion given the name and condType
func (c *MysqlCluster) GetNodeCondition(name string, condType api.NodeConditionType) *api.NodeCondition {
	nodeStatusIndex := c.GetNodeStatusIndex(name)
	condIndex, exists := GetNodeConditionIndex(&c.Status.Nodes[nodeStatusIndex], condType)
	if exists {
		return &c.Status.Nodes[nodeStatusIndex].Conditions[condIndex]
	}

	return nil
}

// GetNodeStatusFor returns the node status for specified hostname
func (c *MysqlCluster) GetNodeStatusFor(name string) api.NodeStatus {
	nodeStatusIndex := c.GetNodeStatusIndex(name)
	return c.Status.Nodes[nodeStatusIndex]
}
