package orchestrator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

type Orchestrator interface {
	Discover(host string, port int) error
	Forget(host string, port int) error

	Master(clusterHint string) (*Instance, error)
	ClusterOSCReplicas(cluster string) ([]Instance, error)
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
	if err := o.makeGetAPIResponse(fmt.Sprintf("discover/%s/%d", host, port)); err != nil {
		return err
	}

	return nil
}

func (o *orchestrator) Forget(host string, port int) error {
	if err := o.makeGetAPIResponse(fmt.Sprintf("forget/%s/%d", host, port)); err != nil {
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

func (o *orchestrator) makeGetAPIResponse(path string) error {
	uri := fmt.Sprintf("%s/%s", o.connectUri, path)
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
		return fmt.Errorf("unmarshal error: %s", err)
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
