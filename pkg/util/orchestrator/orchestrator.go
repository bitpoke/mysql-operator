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

package orchestrator

import (
	"fmt"
)

type Interface interface {
	Discover(host string, port int) error
	Forget(host string, port int) error

	Master(clusterHint string) (*Instance, error)

	Cluster(cluster string) ([]Instance, error)

	AuditRecovery(cluster string) ([]TopologyRecovery, error)
	AckRecovery(id int64, commnet string) error
}

type orchestrator struct {
	connectUri string
}

func NewFromUri(uri string) Interface {
	return &orchestrator{
		connectUri: uri,
	}
}

func (o *orchestrator) Discover(host string, port int) error {
	if err := o.makeGetAPIRequest(fmt.Sprintf("discover/%s/%d", host, port), nil); err != nil {
		return err
	}

	return nil
}

func (o *orchestrator) Forget(host string, port int) error {
	if err := o.makeGetAPIRequest(fmt.Sprintf("forget/%s/%d", host, port), nil); err != nil {
		return err
	}

	return nil
}

func (o *orchestrator) Master(clusterHint string) (*Instance, error) {
	path := fmt.Sprintf("master/%s", clusterHint)
	var inst Instance
	if err := o.makeGetRequest(path, &inst); err != nil {
		return nil, err
	}

	return &inst, nil
}

func (o *orchestrator) Cluster(cluster string) ([]Instance, error) {
	path := fmt.Sprintf("cluster/%s", cluster)
	var insts []Instance
	if err := o.makeGetRequest(path, &insts); err != nil {
		return nil, err
	}

	return insts, nil
}

func (o *orchestrator) AuditRecovery(cluster string) ([]TopologyRecovery, error) {
	path := fmt.Sprintf("audit-recovery/%s", cluster)
	var recoveries []TopologyRecovery
	if err := o.makeGetRequest(path, &recoveries); err != nil {
		return nil, err
	}

	return recoveries, nil
}

func (o *orchestrator) AckRecovery(id int64, comment string) error {
	query := map[string][]string{
		"comment": []string{comment},
	}
	if err := o.makeGetAPIRequest(fmt.Sprintf("ack-recovery/%d", id), query); err != nil {
		return err
	}

	return nil
}
