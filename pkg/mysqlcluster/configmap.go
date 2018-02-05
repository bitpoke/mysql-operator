package mysqlcluster

import (
	"fmt"
	"sort"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *cluster) createConfigMapFiles() apiv1.ConfigMap {
	return apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(ConfigMap),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Data: map[string]string{
			"master.cnf": getConfigFilesStr("mysqld", MysqlMasterConfigs,
				MysqlMasterSlaveConfigs, c.cl.Spec.MysqlConf),
			"slave.cnf": getConfigFilesStr("mysqld", MysqlSlaveConfigs,
				MysqlMasterSlaveConfigs, c.cl.Spec.MysqlConf),
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
