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

package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

type Wrapper struct {
	username []byte
	password []byte
}

func NewWrapper(username, password string) *Wrapper {
	u := sha256.Sum256([]byte(username))
	p := sha256.Sum256([]byte(password))
	return &Wrapper{username: u[:], password: p[:]}
}

const notAuthorized = "Not Authorized"

func (wrapper *Wrapper) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(wrapper, r) {
			http.Error(w, notAuthorized, http.StatusUnauthorized)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func (wrapper *Wrapper) WrapFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(wrapper, r) {
			http.Error(w, notAuthorized, http.StatusUnauthorized)
			return
		}

		handlerFunc(w, r)
	})
}

func authorized(wrapper *Wrapper, r *http.Request) bool {
	username, password, isOk := r.BasicAuth()
	u := sha256.Sum256([]byte(username))
	p := sha256.Sum256([]byte(password))
	return isOk &&
		subtle.ConstantTimeCompare(wrapper.username, u[:]) == 1 &&
		subtle.ConstantTimeCompare(wrapper.password, p[:]) == 1
}
