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

package fake

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	// nolint: golint
	. "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

// OrcFakeClient is a structure that implements orchestrator client interface used in
// testing
type OrcFakeClient struct {
	Clusters   map[string][]*Instance
	Recoveries map[string][]TopologyRecovery
	AckRec     []int64

	Discovered []InstanceKey

	lock *sync.Mutex
}

var nextID int64

func getNextID() int64 {
	nextID = nextID + 1
	return nextID
}

const (
	// NoLag is the constant that sets an instance as no lag
	NoLag int64 = -1
)

// New fake orchestrator client
func New() *OrcFakeClient {
	return &OrcFakeClient{
		lock: &sync.Mutex{},
	}
}

// Reset removes all instances and ack from a client
func (o *OrcFakeClient) Reset() {
	o.Clusters = *new(map[string][]*Instance)
	o.Recoveries = *new(map[string][]TopologyRecovery)
	o.AckRec = []int64{}
	o.Discovered = []InstanceKey{}
}

// AddInstance add a instance to orchestrator client
func (o *OrcFakeClient) AddInstance(instance Instance) *Instance {
	o.lock.Lock()
	defer o.lock.Unlock()

	cluster := instance.ClusterName

	instance.Key.Port = 3306
	instance.MasterKey.Port = 3306

	if o.Clusters == nil {
		o.Clusters = make(map[string][]*Instance)
	}

	clusters, ok := o.Clusters[cluster]
	if ok {
		o.Clusters[cluster] = append(clusters, &instance)
	} else {
		o.Clusters[cluster] = []*Instance{&instance}
	}

	return &instance
}

// RemoveInstance deletes a instance from orchestrator
func (o *OrcFakeClient) RemoveInstance(cluster, host string) {
	o.lock.Lock()
	defer o.lock.Unlock()

	instances, ok := o.Clusters[cluster]
	if !ok {
		return
	}
	index := -1
	for i, inst := range instances {
		if inst.Key.Hostname == host {
			index = i
		}
	}

	if index == -1 {
		return
	}

	// remove instance from cluster
	o.Clusters[cluster] = append(instances[:index], instances[index+1:]...)

	// remove cluster key from map if has no instance
	if len(o.Clusters[cluster]) == 0 {
		delete(o.Clusters, cluster)
	}
}

// AddRecoveries add a recovery for a cluster
func (o *OrcFakeClient) AddRecoveries(cluster string, ack bool) int64 {
	o.lock.Lock()
	defer o.lock.Unlock()

	id := getNextID()
	tr := TopologyRecovery{
		Id:                     id,
		Acknowledged:           ack,
		RecoveryStartTimestamp: "2018-05-16T13:15:05Z",
	}
	rs, ok := o.Recoveries[cluster]
	if ok {
		o.Recoveries[cluster] = append(rs, tr)
	}
	if o.Recoveries == nil {
		o.Recoveries = make(map[string][]TopologyRecovery)
	}
	o.Recoveries[cluster] = []TopologyRecovery{tr}
	return id
}

// CheckAck verify is an ack is present
func (o *OrcFakeClient) CheckAck(id int64) bool {
	o.lock.Lock()
	defer o.lock.Unlock()

	for _, a := range o.AckRec {
		if a == id {
			return true
		}
	}

	return false
}

// CheckDiscovered verify if a key was discovered or not
func (o *OrcFakeClient) CheckDiscovered(key string) bool {
	for _, d := range o.Discovered {
		if d.Hostname == key {
			return true
		}
	}

	return false
}

func (o *OrcFakeClient) getHostClusterAlias(host string) string {
	// input: cluster-1943285891-mysql-0.cluster-1943285891-mysql-nodes.default
	// output: cluster-1943285891.default
	s := strings.Split(host, ".")
	ns := s[2]
	cluster := strings.Replace(s[1], "-mysql-nodes", "", 1)
	return fmt.Sprintf("%s.%s", cluster, ns)
}

// Discover register a host into orchestrator
func (o *OrcFakeClient) Discover(host string, port int) error {
	o.Discovered = append(o.Discovered, InstanceKey{
		Hostname: host,
		Port:     port,
	})

	cluster := o.getHostClusterAlias(host)
	o.AddInstance(Instance{
		ClusterName: cluster,
		Key:         InstanceKey{Hostname: host},
		ReadOnly:    false,
		SlaveLagSeconds: sql.NullInt64{
			Valid: false,
			Int64: 0,
		},
		IsUpToDate:       true,
		IsLastCheckValid: true,
	})

	return nil
}

// Forget removes a host from orchestrator
func (o *OrcFakeClient) Forget(host string, port int) error {
	// determine cluster name
	cluster := o.getHostClusterAlias(host)
	o.RemoveInstance(cluster, host)
	return nil
}

// Master returns the master of a cluster
func (o *OrcFakeClient) Master(clusterHint string) (*Instance, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	insts, ok := o.Clusters[clusterHint]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	for _, inst := range insts {
		if !inst.ReadOnly {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("[faker] master not found")
}

// Cluster returns the list of instances from a cluster
func (o *OrcFakeClient) Cluster(cluster string) ([]Instance, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	instsPointers, ok := o.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	insts := []Instance{}
	for _, instP := range instsPointers {
		insts = append(insts, *instP)
	}

	return insts, nil
}

// AuditRecovery returns recoveries for a cluster
func (o *OrcFakeClient) AuditRecovery(cluster string) ([]TopologyRecovery, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	recoveries, ok := o.Recoveries[cluster]
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	return recoveries, nil
}

// AckRecovery ack a recovery
func (o *OrcFakeClient) AckRecovery(id int64, comment string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.AckRec = append(o.AckRec, id)
	return nil
}

// SetHostWritable make a host writable
func (o *OrcFakeClient) SetHostWritable(key InstanceKey) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	for _, instances := range o.Clusters {
		for _, instance := range instances {
			if instance.Key.Hostname == key.Hostname {
				instance.ReadOnly = false
				return nil
			}
		}
	}
	return fmt.Errorf("the desired host and port was not found")
}

// SetHostReadOnly make a host read only
func (o *OrcFakeClient) SetHostReadOnly(key InstanceKey) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	for _, instances := range o.Clusters {
		for _, instance := range instances {
			if instance.Key.Hostname == key.Hostname {
				instance.ReadOnly = true
				return nil
			}
		}
	}
	return fmt.Errorf("the desired host and port was not found")
}

// BeginMaintenance set a host in maintenance
func (o *OrcFakeClient) BeginMaintenance(key InstanceKey, owner, reason string) error {
	return nil
}

// EndMaintenance set a host in maintenance
func (o *OrcFakeClient) EndMaintenance(key InstanceKey) error {
	return nil
}

// Maintenance put a node into maintenance
func (o *OrcFakeClient) Maintenance() ([]Maintenance, error) {
	return nil, nil
}
