package v1alpha1

import (
	"time"

	"github.com/golang/glog"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		for i, cond := range c.Status.Conditions {
			if cond.Type == condType {
				if cond.Status != newCondition.Status {
					glog.Infof("Found status change for mysql cluster "+
						"%q condition %q: %q -> %q; setting lastTransitionTime to %v",
						c.Name, condType, cond.Status, status, t)
					newCondition.LastTransitionTime = metav1.NewTime(t)
				} else {
					newCondition.LastTransitionTime = cond.LastTransitionTime
				}

				c.Status.Conditions[i] = newCondition
				break
			}
		}
	}
}
