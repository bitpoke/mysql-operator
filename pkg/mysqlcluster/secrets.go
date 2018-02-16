package mysqlcluster

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util"
)

func (f *cFactory) createEnvConfigSecret() apiv1.Secret {
	return apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            f.getNameForResource(EnvSecret),
			Labels:          f.getLabels(map[string]string{}),
			OwnerReferences: f.getOwnerReferences(),
		},
		Data: f.getConfigSecretEnv(),
	}
}

func (f *cFactory) getConfigSecretEnv() map[string][]byte {
	configs := map[string]string{
		"TITANIUM_RELEASE_NAME":      f.cl.Name,
		"TITANIUM_GOVERNING_SERVICE": f.getNameForResource(HeadlessSVC),

		"TITANIUM_INIT_BUCKET_URI":   f.cl.Spec.InitBucketURI,
		"TITANIUM_BACKUP_BUCKET_URI": f.cl.Spec.BackupBucketURI,
	}
	fConf := make(map[string][]byte)
	for k, v := range configs {
		fConf[k] = []byte(v)
	}
	return fConf
}

func (f *cFactory) createDbCredentialSecret(name string) *apiv1.Secret {
	labels := f.getLabels(map[string]string{})
	ownerRs := f.getOwnerReferences()
	scrtClient := f.client.CoreV1().Secrets(f.namespace)
	s, err := scrtClient.Get(name, metav1.GetOptions{})
	if err == nil {
		// if the secret exists then add to it owner reference, and default
		// labels
		labels = f.getLabels(s.Labels)
	}

	newS := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          labels,
			OwnerReferences: ownerRs,
			Namespace:       f.namespace,
		},
		Data: map[string][]byte{},
	}

	return f.updateDbCredentialSecret(newS)
}

// The length of the new generated strings.
const rStrLen = 18

type dbCreds struct {
	User         string
	Password     string
	Database     string
	RootPassword string
	DbConnectURL string
}

func (db *dbCreds) SetDefaults(host string) {
	if len(db.User) == 0 {
		db.User = util.RandomString(rStrLen)
	}
	if len(db.Password) == 0 {
		db.Password = util.RandomString(rStrLen)
	}
	if len(db.Database) == 0 {
		db.Database = util.RandomString(rStrLen)
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

func (f *cFactory) updateDbCredentialSecret(s *apiv1.Secret) *apiv1.Secret {
	creds := newCredsFrom(s.Data)
	creds.SetDefaults(f.getPorHostName(0))
	s.Data = creds.ToData()
	return s
}
