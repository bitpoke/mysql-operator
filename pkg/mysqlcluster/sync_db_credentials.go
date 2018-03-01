package mysqlcluster

import (
	"fmt"

	kcore "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util"
)

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
			db := newCredsFrom(in.Data)
			// TODO: get master pod from ORC
			// and update this secret everytime when a failover is done
			db.SetDefaults(f.getPodHostName(0))
			in.Data = db.ToData()
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

type dbCreds struct {
	User         string
	Password     string
	Database     string
	RootPassword string
	DbConnectURL string
}

// TODO: remove inits for users...
func (db *dbCreds) SetDefaults(host string) {
	if len(db.User) == 0 {
		db.User = util.RandStringUser(rStrLen)
	}
	if len(db.Password) == 0 {
		db.Password = util.RandomString(rStrLen)
	}
	if len(db.Database) == 0 {
		db.Database = util.RandStringUser(rStrLen)
	}
	if len(db.RootPassword) == 0 {
		db.RootPassword = util.RandomString(rStrLen)
	}
	db.DbConnectURL = fmt.Sprintf(
		"mysql://%s:%s@%s/%s",
		db.User, db.Password, host, db.Database,
	)
}

func newCredsFrom(d map[string][]byte) dbCreds {
	c := dbCreds{}
	if v, ok := d["USER"]; ok {
		c.User = string(v)
	}
	if v, ok := d["PASSWORD"]; ok {
		c.Password = string(v)
	}
	if v, ok := d["DATABASE"]; ok {
		c.Database = string(v)
	}
	if v, ok := d["ROOT_PASSWORD"]; ok {
		c.RootPassword = string(v)
	}
	return c
}

func (db *dbCreds) ToData() map[string][]byte {
	return map[string][]byte{
		"USER":           []byte(db.User),
		"PASSWORD":       []byte(db.Password),
		"DATABASE":       []byte(db.Database),
		"ROOT_PASSWORD":  []byte(db.RootPassword),
		"DB_CONNECT_URL": []byte(db.DbConnectURL),
	}
}
