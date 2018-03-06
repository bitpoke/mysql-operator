package orchestrator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Orchestrator interface {
	Discover(host string, port int) error
	Forget(host string, port int) error

	Master(clusterHint string) (string, int, error)
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

func (o *orchestrator) Master(clusterHint string) (string, int, error) {
	port := 3306
	inst, err := o.makeGetInstance(fmt.Sprintf("master/%s", clusterHint))
	if err != nil {
		return "", port, err
	}
	return inst.Key.Hostname, inst.Key.Port, nil
}

func (o *orchestrator) makeGetInstance(path string) (*Instance, error) {
	uri := fmt.Sprintf("%s/%s", o.connectUri, path)
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var inst Instance
	err = json.Unmarshal(body, &inst)
	return &inst, err
}

type APIResponse struct {
	Code    string
	Message string
	Details string
}

func (o *orchestrator) makeGetAPIResponse(path string) error {

	uri := fmt.Sprintf("%s/%s", o.connectUri, path)
	resp, err := http.Get(uri)
	if err != nil {
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var apiObj APIResponse
	err = json.Unmarshal(body, &apiObj)
	if err != nil {
		return err
	}

	switch apiObj.Code {
	case "OK":
		return nil
	case "ERROR":
		return fmt.Errorf("orc msg: %s", apiObj.Message)
	}

	return fmt.Errorf("unknown response code from orc. obj: %v ", apiObj)
}
