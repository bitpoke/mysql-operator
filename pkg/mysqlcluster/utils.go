package mysqlcluster

import (
	"github.com/appscode/kutil"
	core "k8s.io/api/core/v1"
)

func ensureProbe(in *core.Probe, ids, ts, ps int32, handler core.Handler) *core.Probe {
	if in == nil {
		in = &core.Probe{}
	}
	in.InitialDelaySeconds = ids
	in.TimeoutSeconds = ts
	in.PeriodSeconds = ps
	if handler.Exec != nil {
		in.Handler.Exec = handler.Exec
	}
	if handler.HTTPGet != nil {
		in.Handler.HTTPGet = handler.HTTPGet
	}
	if handler.TCPSocket != nil {
		in.Handler.TCPSocket = handler.TCPSocket
	}

	return in
}

func ensureContainerPorts(in []core.ContainerPort, ports ...core.ContainerPort) []core.ContainerPort {
	if len(in) == 0 {
		return ports
	}
	return in
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
