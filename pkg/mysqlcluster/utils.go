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
	"strings"

	"github.com/appscode/kutil"
	kcore "github.com/appscode/kutil/core/v1"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/reference"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

func ensureProbe(in *core.Probe, deply, timeout, period int32, handler core.Handler) *core.Probe {
	if in == nil {
		in = &core.Probe{}
	}
	in.InitialDelaySeconds = deply
	in.TimeoutSeconds = timeout
	in.PeriodSeconds = period
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

func getPodForHostname(client kubernetes.Interface, ns string, lbs labels.Set, hostname string) (*core.Pod, error) {
	selector := labels.SelectorFromSet(lbs)
	podList, err := client.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	//pods, err := f.podLister.List(selector)
	if err != nil {
		return nil, fmt.Errorf("listing pods: %s", err)
	}

	for _, pod := range podList.Items {
		if strings.Contains(hostname, pod.Name) {
			return &pod, nil
		}
	}

	return nil, fmt.Errorf("pod with hostname %s not found", hostname)
}

func podCondIndex(pod *core.Pod, ty core.PodConditionType) (int, bool) {
	for i, cond := range pod.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

// TODO: refactor this method to getEndpointsSubsets and to return just subsets of endpoints
func (f *cFactory) addNodesToService(serviceName string, hosts ...string) error {
	var pods []*core.Pod
	for _, host := range hosts {
		pod, err := getPodForHostname(f.client, f.namespace, f.getLabels(map[string]string{}), host)
		if err != nil {
			glog.Errorf("Failed to set %s service endpoints: %s", serviceName, err)
			continue
		}

		if len(pod.Status.PodIP) == 0 {
			glog.Errorf("Failed to set %s service endpoints, ip for pod %s not set %s", serviceName, pod.Name, err)
			continue
		}
		pods = append(pods, pod)
	}

	if len(pods) == 0 {
		// no need to create endpoints, because will fail without addresses
		glog.V(3).Infof("Configuriong service '%s': Nothing to do for hosts: %v, pods not found!", serviceName, hosts)
		return nil
	}

	meta := metav1.ObjectMeta{
		Name:            serviceName,
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchEndpoints(f.client, meta,
		func(in *core.Endpoints) *core.Endpoints {
			if len(in.Subsets) != 1 {
				in.Subsets = make([]core.EndpointSubset, 1)
			}

			readyAddr := []core.EndpointAddress{}
			notReadyAddr := []core.EndpointAddress{}

			for _, pod := range pods {
				ref, _ := reference.GetReference(runtime.NewScheme(), pod)
				ep := core.EndpointAddress{
					IP:        pod.Status.PodIP,
					TargetRef: ref,
				}

				readyIndex, exists := podCondIndex(pod, core.PodReady)
				if exists && pod.Status.Conditions[readyIndex].Status == core.ConditionTrue {
					readyAddr = append(readyAddr, ep)
				} else {
					notReadyAddr = append(notReadyAddr, ep)
				}
			}

			in.Subsets[0].Addresses = readyAddr
			in.Subsets[0].NotReadyAddresses = notReadyAddr

			if len(in.Subsets[0].Ports) != 1 {
				in.Subsets[0].Ports = make([]core.EndpointPort, 1)
			}
			in.Subsets[0].Ports[0].Name = MysqlPortName
			in.Subsets[0].Ports[0].Port = MysqlPort
			in.Subsets[0].Ports[0].Protocol = "TCP"

			return in
		})

	glog.Infof("Endpoints for service '%s' were %s.", serviceName, getStatusFromKVerb(act))

	return err
}

// Returns name of current master host in a cluster
func (f *cFactory) getMasterHost() string {
	masterHost := f.cluster.GetPodHostname(0)

	for _, ns := range f.cluster.Status.Nodes {
		if cond := ns.GetCondition(api.NodeConditionMaster); cond != nil &&
			cond.Status == core.ConditionTrue {
			masterHost = ns.Name
		}
	}

	return masterHost
}

// getRecoveryTextMsg returns a string human readable for cluster recoveries
func getRecoveryTextMsg(acks []orc.TopologyRecovery) string {
	text := ""
	for _, a := range acks {
		text += fmt.Sprintf(" {id: %d, uid: %s, success: %t, time: %s}",
			a.Id, a.UID, a.IsSuccessful, a.RecoveryStartTimestamp)
	}

	return fmt.Sprintf("[%s]", text)
}
