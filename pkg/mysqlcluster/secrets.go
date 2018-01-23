package mysqlcluster

import (
	"fmt"
	"sort"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		"TITANIUM_BACKUP_BUCKET":     c.cl.Spec.BackupBucketURI, // TODO: change env var
		"TITANIUM_RELEASE_NAME":      c.cl.Name,
		"TITANIUM_GOVERNING_SERVICE": c.getNameForResource(HeadlessSVC),
		"TITANIUM_INIT_BUCKET_URI":   c.cl.Spec.InitBucketURI,

		"TITANIUM_REPLICATION_USER":     c.cl.Spec.MysqlReplicationUser,
		"TITANIUM_REPLICATION_PASSWORD": c.cl.Spec.MysqlReplicationPassword,

		"MYSQL_ROOT_PASSWORD": c.cl.Spec.MysqlRootPassword,
	}

	if len(c.cl.Spec.MysqlUser) != 0 {
		configs["MYSQL_USER"] = c.cl.Spec.MysqlUser
		configs["MYSQL_PASSWORD"] = c.cl.Spec.MysqlPassword
		configs["MYSQL_DATABASE"] = c.cl.Spec.MysqlDatabase
	}

	fConf := make(map[string][]byte)
	for k, v := range configs {
		fConf[k] = []byte(v)
	}
	return fConf
}

func (c *cluster) createConfigMapFiles() apiv1.ConfigMap {
	return apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(ConfigMap),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Data: map[string]string{
			"master.cnf": getConfigFilesStr("mysqld", MysqlMasterConfigs,
				MysqlMasterSlaveConfigs, c.cl.Spec.MysqlConfig),
			"slave.cnf": getConfigFilesStr("mysqld", MysqlSlaveConfigs,
				MysqlMasterSlaveConfigs, c.cl.Spec.MysqlConfig),
		},
	}
}

func getConfigFilesStr(section string, confs ...map[string]string) string {
	var strCnf []string
	for _, conf := range confs {
		for k, v := range conf {
			strCnf = append(strCnf, fmt.Sprintf("%s = %s", k, v))
		}
	}
	// sort to be always the same when calling deepEqual
	sort.Strings(strCnf)

	return fmt.Sprintf("\n[%s]\n%s\n", section, strings.Join(strCnf, "\n"))
}
