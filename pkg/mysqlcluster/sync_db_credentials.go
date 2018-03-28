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
		state = statusFaild
		return
	}
	secret, err := f.client.CoreV1().Secrets(f.namespace).Get(
		f.cluster.Spec.SecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		err = fmt.Errorf("secret '%s' failed to get: %s", f.cluster.Spec.SecretName, err)
		state = statusFaild
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
