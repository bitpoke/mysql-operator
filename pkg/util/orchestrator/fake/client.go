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
	"fmt"

	. "github.com/presslabs/mysql-operator/pkg/util/orchestrator"
)

type FakeOrc struct {
	Clusters   map[string][]*Instance
	Recoveries map[string][]TopologyRecovery
	AckRec     []int64

	Discovered []InstanceKey
}

func New() *FakeOrc {
	return &FakeOrc{}
}

func (o *FakeOrc) AddInstance(cluster, host string, master bool, lag int64, slaveRunning, upToDate bool) *Instance {
	valid := true
	if lag < 0 {
		valid = false
	}
	inst := &Instance{
		Key: InstanceKey{
			Hostname: host,
			Port:     3306,
		},
		ReadOnly: !master,
		SlaveLagSeconds: NullInt64{
			Valid: valid,
			Int64: lag,
		},
		ClusterName:       cluster,
		Slave_SQL_Running: slaveRunning,
		Slave_IO_Running:  slaveRunning,
		IsUpToDate:        upToDate,
		IsLastCheckValid:  upToDate,
	}
	if o.Clusters == nil {
		o.Clusters = make(map[string][]*Instance)
	}
	clusters, ok := o.Clusters[cluster]
	if ok {
		o.Clusters[cluster] = append(clusters, inst)
	} else {
		o.Clusters[cluster] = []*Instance{inst}
	}

	return inst
}

func (o *FakeOrc) RemoveInstance(cluster, host string) {
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

	o.Clusters[cluster] = append(instances[:index], instances[index+1:]...)
}

func (o *FakeOrc) AddRecoveries(cluster string, id int64, ack bool) {
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
}

func (o *FakeOrc) CheckAck(id int64) bool {
	for _, a := range o.AckRec {
		if a == id {
			return true
		}
	}

	return false
}

func (o *FakeOrc) CheckDiscovered(key string) bool {
	for _, d := range o.Discovered {
		if d.Hostname == key {
			return true
		}
	}

	return false
}

func (o *FakeOrc) Discover(host string, port int) error {
	o.Discovered = append(o.Discovered, InstanceKey{
		Hostname: host,
		Port:     port,
	})
	return nil
}

func (o *FakeOrc) Forget(host string, port int) error {
	return nil
}

func (o *FakeOrc) Master(clusterHint string) (*Instance, error) {
	insts, ok := o.Clusters[clusterHint]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	for _, inst := range insts {
		if !inst.ReadOnly {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("[faker] master not found!!!!")
}

func (o *FakeOrc) Cluster(cluster string) ([]Instance, error) {
	insts, ok := o.Clusters[cluster]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	var Insts []Instance
	for _, inst := range insts {
		Insts = append(Insts, *inst)
	}
	return Insts, nil
}

func (o *FakeOrc) ForgetCluster(cluster string) error {
	return nil
}

func (o *FakeOrc) AuditRecovery(cluster string) ([]TopologyRecovery, error) {
	recoveries, ok := o.Recoveries[cluster]
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	return recoveries, nil
}

func (o *FakeOrc) AckRecovery(id int64, comment string) error {
	o.AckRec = append(o.AckRec, id)
	return nil
}

func (o *FakeOrc) SetHostWritable(key InstanceKey) error {

	check := false
	for _, instances := range o.Clusters {
		for _, instance := range instances {
			if instance.Key.Hostname == key.Hostname && instance.Key.Port == key.Port {
				instance.ReadOnly = false
				check = true
			}
		}
	}
	if check == true {
		return nil
	} else {
		return fmt.Errorf("the desired host and port was not found")
	}
}

func (o *FakeOrc) SetHostReadOnly(key InstanceKey) error {

	check := false
	for _, instances := range o.Clusters {
		for _, instance := range instances {
			if instance.Key.Hostname == key.Hostname && instance.Key.Port == key.Port {
				instance.ReadOnly = true
				check = true
			}
		}
	}
	if check == true {
		return nil
	} else {
		return fmt.Errorf("the desired host and port was not found")
	}
}

func (o *FakeOrc) BeginMaintenance(key InstanceKey, owner, reason string) error {
	return nil
}

func (o *FakeOrc) EndMaintenance(key InstanceKey) error {
	return nil
}
