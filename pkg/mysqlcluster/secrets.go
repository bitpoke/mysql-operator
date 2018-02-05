package mysqlcluster

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util"
)

func (c *cluster) createEnvConfigSecret() apiv1.Secret {
	return apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(EnvSecret),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Data: c.getConfigSecretEnv(),
	}
}

func (c *cluster) getConfigSecretEnv() map[string][]byte {
	configs := map[string]string{
		"TITANIUM_RELEASE_NAME":      c.cl.Name,
		"TITANIUM_GOVERNING_SERVICE": c.getNameForResource(HeadlessSVC),

		"TITANIUM_INIT_BUCKET_URI":   c.cl.Spec.InitBucketURI,
		"TITANIUM_BACKUP_BUCKET_URI": c.cl.Spec.BackupBucketURI,

		//		"TITANIUM_REPLICATION_USER":     c.cl.Spec.MysqlReplicationUser,
		//		"TITANIUM_REPLICATION_PASSWORD": c.cl.Spec.MysqlReplicationPassword,

		//		"MYSQL_ROOT_PASSWORD": c.cl.Spec.MysqlRootPassword,
	}

	//	if len(c.cl.Spec.MysqlUser) != 0 {
	//		configs["MYSQL_USER"] = c.cl.Spec.MysqlUser
	//		configs["MYSQL_PASSWORD"] = c.cl.Spec.MysqlPassword
	//		configs["MYSQL_DATABASE"] = c.cl.Spec.MysqlDatabase
	//	}

	fConf := make(map[string][]byte)
	for k, v := range configs {
		fConf[k] = []byte(v)
	}
	return fConf
}

func (c *cluster) createDbCredentialSecret() *apiv1.Secret {
	s := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(DbSecret),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Data: map[string][]byte{},
	}

	return c.updateDbCredentialSecret(s)
}

// The length of the new generated strings.
const rStrLen = 16

type dbCreds struct {
	User         string
	Password     string
	Database     string
	RootPassword string
	ReplicaUser  string
	ReplicaPass  string
	DbConnectUrl string
}

func (c *dbCreds) SetDefaults(host string) {
	if len(c.User) == 0 {
		c.User = util.RandomString(rStrLen)
	}
	if len(c.Password) == 0 {
		c.Password = util.RandomString(rStrLen)
	}
	if len(c.Database) == 0 {
		c.Database = util.RandomString(rStrLen)
	}
	if len(c.ReplicaUser) == 0 {
		c.ReplicaUser = util.RandomString(rStrLen)
	}
	if len(c.ReplicaPass) == 0 {
		c.ReplicaPass = util.RandomString(rStrLen)
	}
	c.DbConnectUrl = fmt.Sprintf(
		"mysql://%s:%s@%s/%s",
		c.User, c.Password, host, c.Database,
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
	if v, ok := d["REPLICATION_USER"]; ok {
		c.ReplicaUser = string(v)
	}
	if v, ok := d["REPLICATION_PASSWORD"]; ok {
		c.ReplicaPass = string(v)
	}
	return c
}

func (c *dbCreds) ToData() map[string][]byte {
	return map[string][]byte{
		"USER":                 []byte(c.User),
		"PASSWORD":             []byte(c.Password),
		"DATABASE":             []byte(c.Database),
		"ROOT_PASSWORD":        []byte(c.RootPassword),
		"REPLICATION_USER":     []byte(c.ReplicaUser),
		"REPLICATION_PASSWORD": []byte(c.ReplicaPass),
		"DB_CONNECT_URL":       []byte(c.DbConnectUrl),
	}
}

func (c *cluster) updateDbCredentialSecret(s *apiv1.Secret) *apiv1.Secret {
	creds := newCredsFrom(s.Data)
	creds.SetDefaults(c.getPorHostName(0))
	s.Data = creds.ToData()
	return s
}
