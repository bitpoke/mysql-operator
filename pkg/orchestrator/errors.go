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
	"strings"
)

// Error contains orchestrator error details
type Error struct {
	HTTPStatus int
	Path       string
	Message    string
	Details    interface{}
}

func (e Error) Error() string {
	return fmt.Sprintf("[orc]: status: %d path: %s msg: %s, details: %v",
		e.HTTPStatus, e.Path, e.Message, e.Details)
}

// NewError returns a specific orchestrator error with extra details
func NewError(resp *http.Response, path string, details interface{}) *Error {
	rsp := &Error{
		HTTPStatus: resp.StatusCode,
		Path:       path,
		Details:    details,
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rsp.Message = "<<Can't read body>>"
		return rsp
	}

	if err = json.Unmarshal(body, rsp); err != nil {
		log.V(-1).Info("error when unmarhal error data", "body", string(body))
		rsp.Message = fmt.Sprintf("<<can't get more details, in error: error: %s, body: %s>>", err, body)
		return rsp
	}

	return rsp
}

// NewErrorMsg returns an orchestrator error with extra msg
func NewErrorMsg(msg string, path string) *Error {
	return &Error{
		HTTPStatus: 0,
		Message:    msg,
		Path:       path,
	}
}

// IsNotFound checks if the given error is orchestrator error and it's cluster not found.
func IsNotFound(err error) bool {
	if orcErr, ok := err.(*Error); ok {
		if strings.Contains(orcErr.Message, "Unable to determine cluster name") {
			return true
		}

		// When querying for instances orchestrator returns the following error message when the
		// replica cannot be reached.
		// https://github.com/github/orchestrator/blob/151029a103429fe16123b9842d1a5b4b175bd5d5/go/http/api.go#L184
		if strings.Contains(orcErr.Message, "Cannot read instance") {
			return true
		}

		// https://github.com/github/orchestrator/blob/7bef26f042aafbd956daeaede0cd4aab2ba46e65/go/http/api.go#L1949
		if strings.Contains(orcErr.Message, "No masters found") {
			return true
		}
	}
	return false
}
