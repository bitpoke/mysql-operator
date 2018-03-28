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
	"k8s.io/apimachinery/pkg/util/runtime"
)

type dbCreds struct {
	User         string
	Password     string
	Database     string
	RootPassword string
}

func (f *cFactory) syncDbCredentialsSecret() (state string, err error) {
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
			var creds dbCreds
			if _, ok := in.Data["ROOT_PASSWORD"]; !ok {
				runtime.HandleError(fmt.Errorf("ROOT_PASSWORD not set in secret: %s", in.Name))
				return in
			}

			creds.RootPassword = string(in.Data["ROOT_PASSWORD"])
			creds.User = "root"
			creds.Password = creds.RootPassword
			creds.Database = ""

			u, oku := in.Data["USER"]
			p, okp := in.Data["PASSWORD"]
			if oku && okp {
				creds.User = string(u)
				creds.Password = string(p)
			}
			if d, ok := in.Data["DATABASE"]; ok {
				creds.Database = string(d)
			}

			in.Data["DB_CONNECT_URL"] = []byte(fmt.Sprintf(
				"mysql://%s:%s@%s/%s",
				creds.User, creds.Password, f.cluster.GetMasterHost(), creds.Database,
			))

			return in
		})

	state = getStatusFromKVerb(act)
	return
}
