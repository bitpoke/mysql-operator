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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("orchestrator.client")

func (o *orchestrator) makeGetRequest(path string, out interface{}) *Error {
	uri := fmt.Sprintf("%s/%s", o.connectURI, path)
	log.V(2).Info("orchestrator request info", "uri", uri, "outobj", out)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return NewErrorMsg(fmt.Sprintf("can't create request: %s", err.Error()), path)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return NewErrorMsg(err.Error(), path)
	}

	if resp.StatusCode >= 500 {
		return NewError(resp, path, nil)
	}

	if err := unmarshalJSON(resp.Body, out); err != nil {
		return NewError(resp, path, err)
	}

	return nil
}

func (o *orchestrator) makeGetAPIRequest(path string, query map[string][]string) error {
	args := url.Values(query).Encode()
	if len(args) != 0 {
		args = "?" + args
	}

	path = fmt.Sprintf("%s%s", path, args)
	var apiObj struct {
		Code    string
		Message string
	}
	if err := o.makeGetRequest(path, &apiObj); err != nil {
		return err
	}

	if apiObj.Code != "OK" {
		return fmt.Errorf("orc failed with: %s", apiObj.Message)
	}

	return nil
}

func unmarshalJSON(in io.Reader, obj interface{}) error {
	body, err := ioutil.ReadAll(in)
	if err != nil {
		return fmt.Errorf("io read error: %s", err)
	}

	if err = json.Unmarshal(body, obj); err != nil {
		log.V(1).Info("error unmarshal data", "body", string(body))
		return err
	}

	return nil
}
