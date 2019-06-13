// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middlewares

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
)

const apiVersionInvalidKey = "broker-api-version-invalid"

type APIVersionMiddleware struct {
	LoggerFactory lager.Logger
}

type ErrorResponse struct {
	Description string
}

func (m APIVersionMiddleware) ValidateAPIVersionHdr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := m.LoggerFactory.Session("version-header-check", lager.Data{})

		err := checkBrokerAPIVersionHdr(req)
		if err != nil {
			logger.Error(apiVersionInvalidKey, err)

			w.Header().Set("Content-type", "application/json")

			statusResponse := http.StatusPreconditionFailed
			w.WriteHeader(statusResponse)
			errorResp := ErrorResponse{
				Description: err.Error(),
			}
			err = json.NewEncoder(w).Encode(errorResp)
			if err != nil {
				logger.Error("encoding response", err, lager.Data{"status": statusResponse, "response": errorResp})
			}

			return
		}

		next.ServeHTTP(w, req)
	})
}

func checkBrokerAPIVersionHdr(req *http.Request) error {
	var version struct {
		Major int
		Minor int
	}
	apiVersion := req.Header.Get("X-Broker-API-Version")
	if apiVersion == "" {
		return errors.New("X-Broker-API-Version Header not set")
	}
	if n, err := fmt.Sscanf(apiVersion, "%d.%d", &version.Major, &version.Minor); err != nil || n < 2 {
		return errors.New("X-Broker-API-Version Header must contain a version")
	}

	if version.Major != 2 {
		return errors.New("X-Broker-API-Version Header must be 2.x")
	}
	return nil
}
