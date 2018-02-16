package mysqlcluster

import (
	"github.com/appscode/kutil"
	"k8s.io/api/core/v1"
)

func getLivenessProbe() *v1.Probe {
	return &v1.Probe{
		InitialDelaySeconds: 30,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{
					"mysqladmin",
					"--defaults-file=/etc/mysql/client.cnf",
					"ping",
				},
			},
		},
	}
}

func getReadinessProbe() *v1.Probe {
	return &v1.Probe{
		InitialDelaySeconds: 5,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{
					"mysql",
					"--defaults-file=/etc/mysql/client.cnf",
					"-e",
					"SELECT 1",
				},
			},
		},
	}
}

func getStatusFromKVerb(verb kutil.VerbType) string {
	switch verb {
	case kutil.VerbUnchanged:
		return statusUpToDate
	case kutil.VerbCreated:
		return statusCreated
	case kutil.VerbPatched, kutil.VerbUpdated, kutil.VerbDeleted:
		return statusUpdated
	default:
		return statusSkip
	}
}
