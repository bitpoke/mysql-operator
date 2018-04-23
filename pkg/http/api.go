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

package http

import (
	"fmt"
	"net/http"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"

	"github.com/presslabs/mysql-operator/pkg/mysqlcluster"
)

type HttpAPI struct{}

var API HttpAPI = HttpAPI{}

func (this *HttpAPI) Health(r render.Render) {
	r.JSON(http.StatusOK, "OK")
}

func (this *HttpAPI) Master(params martini.Params, r render.Render) {
	clusterName := params["cluster"]
	cluster, ok := mysqlcluster.SavedClusters[clusterName]
	if !ok {
		r.JSON(http.StatusNotFound, "404")
		return
	}
	r.JSON(http.StatusOK, cluster.MasterHostname)
}

func (this *HttpAPI) registerAPIEndpoint(m *martini.ClassicMartini,
	path string, handler martini.Handler) {
	fullPath := fmt.Sprintf("/%s", path)

	m.Get(fullPath, handler)
}

// RegisterEndpoints define all api endpoints
func (this *HttpAPI) RegisterEndpoints(m *martini.ClassicMartini) {
	this.registerAPIEndpoint(m, "health", this.Health)
	this.registerAPIEndpoint(m, "master/:cluster", this.Master)
}
