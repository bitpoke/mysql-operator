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
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	kutil "github.com/presslabs/mysql-operator/pkg/util/kube"
	orc "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

type ClusterInfo struct {
	api.MysqlCluster

	// Master represent the cluster master hostname
	MasterHostname string
}

var SavedClusters map[string]ClusterInfo

func init() {
	SavedClusters = make(map[string]ClusterInfo)
}

func (f *cFactory) registerNodesInOrc() error {
	// Register nodes in orchestrator
	if len(f.cluster.Spec.GetOrcUri()) != 0 {
		// try to discover ready nodes into orchestrator
		client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
		for i := 0; i < int(f.cluster.Status.ReadyNodes); i++ {
			host := f.getHostForReplica(i)
			if err := client.Discover(host, MysqlPort); err != nil {
				glog.Warningf("Failed to register %s with orchestrator: %s", host, err.Error())
			}
		}
	}

	return nil
}

func (f *cFactory) updateMasterServiceEndpoints() error {
	masterHost := f.cluster.GetPodHostName(0)

	if len(f.cluster.Spec.GetOrcUri()) != 0 {
		// try to discover ready nodes into orchestrator
		client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
		orcClusterName := fmt.Sprintf("%s.%s", f.cluster.Name, f.cluster.Namespace)
		if inst, err := client.Master(orcClusterName); err == nil {
			masterHost = inst.Key.Hostname
		} else {
			glog.Warningf(
				"Failed getting master for %s: %s, falling back to default.",
				orcClusterName, err,
			)
		}
	}

	SavedClusters[f.cluster.Name] = ClusterInfo{
		MysqlCluster:   *f.cluster.DeepCopy(),
		MasterHostname: masterHost,
	}

	masterPod, err := f.getPodForHostname(masterHost)
	if err != nil {
		glog.Errorf("Failed to set master service endpoints: %s", err)
		return nil
	}

	if len(masterPod.Status.PodIP) == 0 {
		glog.Errorf("Failed to set master service endpoints, ip for pod %s not set %s", masterPod.Name, err)
		return nil
	}

	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.MasterService),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kutil.CreateOrPatchEndpoints(f.client, meta,
		func(in *core.Endpoints) *core.Endpoints {
			if len(in.Subsets) != 1 {
				in.Subsets = make([]core.EndpointSubset, 1)
			}

			ref, _ := reference.GetReference(runtime.NewScheme(), masterPod)

			addresses := []core.EndpointAddress{
				core.EndpointAddress{
					IP:        masterPod.Status.PodIP,
					TargetRef: ref,
				},
			}
			readyIndex, exists := condIndex(masterPod, core.PodReady)
			if exists && masterPod.Status.Conditions[readyIndex].Status == core.ConditionTrue {
				in.Subsets[0].Addresses = addresses
				in.Subsets[0].NotReadyAddresses = nil
			} else {
				in.Subsets[0].Addresses = nil
				in.Subsets[0].NotReadyAddresses = addresses
			}

			if len(in.Subsets[0].Ports) != 1 {
				in.Subsets[0].Ports = make([]core.EndpointPort, 1)
			}
			in.Subsets[0].Ports[0].Name = MysqlPortName
			in.Subsets[0].Ports[0].Port = MysqlPort
			in.Subsets[0].Ports[0].Protocol = "TCP"

			return in
		})

	glog.Infof("Endpoints for service '%s' were %s.",
		f.cluster.GetNameForResource(api.MasterService), getStatusFromKVerb(act))

	return err
}

func (f *cFactory) getPodForHostname(hostname string) (*core.Pod, error) {
	selector := labels.SelectorFromSet(f.getLabels(map[string]string{}))
	podList, err := f.client.CoreV1().Pods(f.namespace).List(metav1.ListOptions{
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

	return nil, fmt.Errorf("pod whith hostname %s not found", hostname)
}

func condIndex(pod *core.Pod, ty core.PodConditionType) (int, bool) {
	for i, cond := range pod.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}

func (f *cFactory) autoAcknowledge() error {
	if len(f.cluster.Spec.GetOrcUri()) == 0 {
		// nothing to do, orchestrator uri not set
		return nil
	}

	i, find := condIndexCluster(f.cluster, api.ClusterConditionReady)
	if !find || f.cluster.Status.Conditions[i].Status != core.ConditionTrue {
		glog.Warning("[autoAcknowledge]: Cluster is not ready for ack.")
		return nil
	}

	if time.Since(f.cluster.Status.Conditions[i].LastTransitionTime.Time).Minutes() < 10 {
		glog.Warning(
			"[autoAcknowledge]: Stateful set is not ready more then 10 minutes. Don't ack.",
		)
		return nil
	}

	// proceed with cluster recovery
	client := orc.NewFromUri(f.cluster.Spec.GetOrcUri())
	recoveries, err := client.AuditRecovery(f.cluster.Name)
	if err != nil {
		return fmt.Errorf("orchestrator audit: %s", err)
	}

	for _, recovery := range recoveries {
		if !recovery.Acknowledged {
			// skip if it's a new recovery, recovery should be older then 10 minutes
			startTime, err := time.Parse(time.RFC3339, recovery.RecoveryStartTimestamp)
			if err != nil {
				glog.Errorf("[autoAcknowledge] Can't parse time: %s for audit recovery: %d",
					err, recovery.Id,
				)
				continue
			}
			if time.Since(startTime).Minutes() < 10 {
				// skip this recovery
				continue
			}

			comment := fmt.Sprintf("Statefulset '%s' is healty more then 10 minutes",
				f.cluster.GetNameForResource(api.StatefulSet),
			)
			if err := client.AckRecovery(recovery.Id, comment); err != nil {
				glog.Errorf("Trying to ack recovery with id %d but failed with error: %s",
					recovery.Id, err,
				)
			}
		}
	}

	return nil
}

func condIndexCluster(r *api.MysqlCluster, ty api.ClusterConditionType) (int, bool) {
	for i, cond := range r.Status.Conditions {
		if cond.Type == ty {
			return i, true
		}
	}

	return 0, false
}
