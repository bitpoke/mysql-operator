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
	"strconv"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/presslabs/mysql-operator/pkg/util"
)

func (f *cFactory) syncClusterSecret() (state string, err error) {
	state = statusUpToDate
	if len(f.cluster.Spec.SecretName) == 0 {
		err = fmt.Errorf("the Spec.SecretName is empty")
		state = statusFailed
		return
	}
	secret, err := f.client.CoreV1().Secrets(f.namespace).Get(
		f.cluster.Spec.SecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		err = fmt.Errorf("secret '%s' failed to get: %s", f.cluster.Spec.SecretName, err)
		state = statusFailed
		return
	}

	_, act, err := kcore.PatchSecret(f.client, secret,
		func(in *core.Secret) *core.Secret {
			if _, ok := in.Data["ROOT_PASSWORD"]; !ok {
				runtime.HandleError(fmt.Errorf("ROOT_PASSWORD not set in secret: %s", in.Name))
				return in
			}

			if len(in.Data["REPLICATION_USER"]) == 0 {
				in.Data["REPLICATION_USER"] = []byte("repl_" + util.RandStringUser(5))
			}
			if len(in.Data["REPLICATION_PASSWORD"]) == 0 {
				in.Data["REPLICATION_PASSWORD"] = []byte(util.RandomString(rStrLen))
			}
			if len(in.Data["METRICS_EXPORTER_USER"]) == 0 {
				in.Data["METRICS_EXPORTER_USER"] = []byte("repl_" + util.RandStringUser(5))
			}
			if len(in.Data["METRICS_EXPORTER_PASSWORD"]) == 0 {
				in.Data["METRICS_EXPORTER_PASSWORD"] = []byte(util.RandomString(rStrLen))
			}
			in.Data["ORC_TOPOLOGY_USER"] = []byte(f.opt.OrchestratorTopologyUser)
			in.Data["ORC_TOPOLOGY_PASSWORD"] = []byte(f.opt.OrchestratorTopologyPassword)

			hash, err := hashstructure.Hash(in.Data, nil)
			if err != nil {
				glog.Errorf("Can't compute the hash for db secret: %s", err)
			}

			if len(in.ObjectMeta.Annotations) == 0 {
				in.ObjectMeta.Annotations = make(map[string]string)
			}

			f.secretHash = strconv.FormatUint(hash, 10)
			in.ObjectMeta.Annotations["secret_hash"] = f.secretHash

			return in
		})

	state = getStatusFromKVerb(act)
	return
}
