package mysqlcluster

import (
	"fmt"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	orc "github.com/presslabs/titanium/pkg/util/orchestrator"
)

type dbCreds struct {
	User         string
	Password     string
	Database     string
	RootPassword string
}

func (f *cFactory) syncDbCredentialsSecret() (state string, err error) {
	state = statusUpToDate
	if len(f.cl.Spec.SecretName) == 0 {
		err = fmt.Errorf("the Spec.SecretName is empty")
		state = statusFaild
		return
	}
	meta := metav1.ObjectMeta{
		Name:      f.cl.Spec.SecretName,
		Labels:    f.getLabels(map[string]string{}),
		Namespace: f.namespace,
	}

	_, act, err := kcore.CreateOrPatchSecret(f.client, meta,
		func(in *core.Secret) *core.Secret {
			var creds dbCreds
			if _, ok := in.Data["ROOT_PASSWORD"]; !ok {
				glog.Errorf("ROOT_PASSWORD not set in secret: %s/%s", in.Namespace, in.Name)
				panic(fmt.Sprintf("ROOT_PASSWORD not set in secret: %s", in.Name))
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

			// TODO: always update it with the master host
			if _, ok := in.Data["DB_CONNECT_URL"]; !ok {
				masterHost := f.getPodHostName(0)
				// connect to orc and get the master host of the cluster.
				if len(f.cl.Spec.GetOrcUri()) != 0 {
					client := orc.NewFromUri(f.cl.Spec.GetOrcUri())
					if inst, err := client.Master(f.cl.Name); err == nil {
						masterHost = inst.Key.Hostname
					}
				}

				in.Data["DB_CONNECT_URL"] = []byte(fmt.Sprintf(
					"mysql://%s:%s@%s/%s",
					creds.User, creds.Password, masterHost, creds.Database,
				))
			}
			return in
		})

	state = getStatusFromKVerb(act)
	return
}
