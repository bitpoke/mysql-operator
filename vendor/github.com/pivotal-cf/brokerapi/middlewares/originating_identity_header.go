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
	"context"
	"net/http"
)

const (
	originatingIdentityKey = "originatingIdentity"
)

func AddOriginatingIdentityToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		originatingIdentity := req.Header.Get("X-Broker-API-Originating-Identity")
		newCtx := context.WithValue(req.Context(), originatingIdentityKey, originatingIdentity)
		next.ServeHTTP(w, req.WithContext(newCtx))
	})
}
