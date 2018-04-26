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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/golang/glog"
)

type Orchestrator interface {
	Discover(host string, port int) error
	Forget(host string, port int) error

	Master(clusterHint string) (*Instance, error)
	ClusterOSCReplicas(cluster string) ([]Instance, error)
	AuditRecovery(cluster string) ([]TopologyRecovery, error)
	AckRecovery(id int64, commnet string) error
}

type orchestrator struct {
	connectUri string
}

func NewFromUri(uri string) Orchestrator {
	return &orchestrator{
		connectUri: uri,
	}
}

func (o *orchestrator) Discover(host string, port int) error {
	if err := o.makeGetAPIResponse(fmt.Sprintf("discover/%s/%d", host, port), nil); err != nil {
		return err
	}

	return nil
}

func (o *orchestrator) Forget(host string, port int) error {
	if err := o.makeGetAPIResponse(fmt.Sprintf("forget/%s/%d", host, port), nil); err != nil {
		return err
	}

	return nil
}

func (o *orchestrator) Master(clusterHint string) (*Instance, error) {
	return o.makeGetInstance(fmt.Sprintf("master/%s", clusterHint))
}

func (o *orchestrator) makeGetInstance(path string) (*Instance, error) {
	uri := fmt.Sprintf("%s/%s", o.connectUri, path)
	glog.V(2).Infof("Orc request on: %s", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error code: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io read error: %s", err)
	}

	var inst Instance
	if err = json.Unmarshal(body, &inst); err != nil {
		glog.V(3).Infof("Unmarshal data is: %s", string(body))
		return nil, fmt.Errorf("unmarshal error: %s", err)
	}

	return &inst, nil
}

type APIResponse struct {
	Code    string
	Message string
	// Detials json.RawMessage
}

func (o *orchestrator) makeGetAPIResponse(path string, query map[string][]string) error {
	args := url.Values(query).Encode()
	if len(args) != 0 {
		args = "?" + args
	}

	uri := fmt.Sprintf("%s/%s%s", o.connectUri, path, args)
	glog.V(2).Infof("Orc request on: %s", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("http get failed: %s", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("http error code: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("io read failed: %s", err)
	}

	var apiObj APIResponse
	err = json.Unmarshal(body, &apiObj)
	if err != nil {
		glog.V(3).Infof("Unmarshal data is: %s", string(body))
		glog.Errorf("[makeGetAPIResponse]: Orchestrator unmarshal data error: %s", err)
		return nil
	}

	switch apiObj.Code {
	case "OK":
		return nil
	case "ERROR":
		return fmt.Errorf("orc msg: %s", apiObj.Message)
	}

	return fmt.Errorf("unknown response code from orc. obj: %v ", apiObj)
}

func (o *orchestrator) ClusterOSCReplicas(cluster string) ([]Instance, error) {
	uri := fmt.Sprintf("%s/cluster-osc-slaves/%s", o.connectUri, cluster)
	glog.V(2).Infof("Orc request on: %s", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error code: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io read error: %s", err)
	}

	var insts []Instance
	if err = json.Unmarshal(body, &insts); err != nil {
		glog.V(3).Infof("Unmarshal data is: %s", string(body))
		return nil, fmt.Errorf("unmarshal error: %s", err)
	}

	return insts, nil
}

func (o *orchestrator) AuditRecovery(cluster string) ([]TopologyRecovery, error) {
	uri := fmt.Sprintf("%s/audit-recovery/%s", o.connectUri, cluster)
	glog.V(2).Infof("Orc request on: %s", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error code: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io read error: %s", err)
	}

	var recoveries []TopologyRecovery
	if err = json.Unmarshal(body, &recoveries); err != nil {
		glog.V(3).Infof("Unmarshal data is: %s", string(body))
		return nil, fmt.Errorf("unmarshal error: %s", err)
	}

	return recoveries, nil
}

func (o *orchestrator) AckRecovery(id int64, comment string) error {
	query := map[string][]string{
		"comment": []string{comment},
	}
	if err := o.makeGetAPIResponse(fmt.Sprintf("ack-recovery/%d", id), query); err != nil {
		return err
	}

	return nil
}
