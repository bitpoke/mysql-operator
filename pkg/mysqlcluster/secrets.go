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

package mysqlcluster

import (
	"fmt"

	kcore "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util"
)

func (f *cFactory) syncEnvSecret() (state string, err error) {

	meta := metav1.ObjectMeta{
		Name: f.cluster.GetNameForResource(api.EnvSecret),
		Labels: f.getLabels(map[string]string{
			"generated": "true"}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchSecret(f.client, meta,
		func(in *core.Secret) *core.Secret {
			in.Data = f.getEnvSecretData(in.Data)
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getEnvSecretData(data map[string][]byte) map[string][]byte {
	rUser := []byte("repl_" + util.RandStringUser(5))
	if u, ok := data["TITANIUM_REPLICATION_USER"]; ok && len(u) > 0 {
		rUser = u
	}
	rPass := []byte(util.RandomString(rStrLen))
	if p, ok := data["TITANIUM_REPLICATION_PASSWORD"]; ok && len(p) > 0 {
		rPass = p
	}

	eUser := []byte("exporter_" + util.RandStringUser(5))
	if u, ok := data["TITANIUM_EXPORTER_USER"]; ok && len(u) > 0 {
		eUser = u
	}
	ePass := []byte(util.RandomString(rStrLen))
	if p, ok := data["TITANIUM_EXPORTER_PASSWORD"]; ok && len(p) > 0 {
		ePass = p
	}

	configs := map[string]string{
		"TITANIUM_HEADLESS_SERVICE": f.cluster.GetNameForResource(api.HeadlessSVC),
		"TITANIUM_INIT_BUCKET_URI":  f.cluster.Spec.InitBucketURI,
		"TITANIUM_ORC_URI":          f.cluster.Spec.GetOrcUri(),

		"DATA_SOURCE_NAME": fmt.Sprintf("%s:%s@(127.0.0.1:%d)/", eUser, ePass, MysqlPort),
	}
	fConf := make(map[string][]byte)
	for k, v := range configs {
		fConf[k] = []byte(v)
	}

	fConf["TITANIUM_REPLICATION_USER"] = rUser
	fConf["TITANIUM_REPLICATION_PASSWORD"] = rPass

	fConf["TITANIUM_EXPORTER_USER"] = eUser
	fConf["TITANIUM_EXPORTER_PASSWORD"] = ePass
	return fConf
}
