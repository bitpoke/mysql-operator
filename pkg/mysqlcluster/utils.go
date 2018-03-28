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
