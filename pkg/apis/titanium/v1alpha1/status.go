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
		if i, exist := condExists(condType, c.Status.Conditions); exist {
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

func condExists(ty ClusterConditionType, cs []ClusterCondition) (int, bool) {
	for i, cond := range cs {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

// Mysql events reason
const (
	EventReasonInitDefaults         = "InitDefaults"
	EventReasonInitDefaultsFaild    = "InitDefaultsFaild"
	EventReasonDbSecretUpdated      = "DbSecretUpdated"
	EventReasonDbSecretFaild        = "DbSecretFaild"
	EventReasonUtilitySecretFaild   = "UtilitySecretFaild"
	EventReasonUtilitySecretUpdated = "UtilitySecretUpdated"
	EventReasonEnvSecretFaild       = "EnvSecretFaild"
	EventReasonEnvSecretUpdated     = "EnvSecretUpdated"
	EventReasonConfigMapFaild       = "MysqlConfigMapFaild"
	EventReasonConfigMapUpdated     = "MysqlConfigMapUpdated"
	EventReasonServiceFaild         = "HLServiceFaild"
	EventReasonServiceUpdated       = "HLServiceUpdated"
	EventReasonSFSFaild             = "SFSFaild"
	EventReasonSFSUpdated           = "SFSUpdated"
)

// Event types
const (
	EventNormal  = "Normal"
	EventWarning = "Warning"
)
