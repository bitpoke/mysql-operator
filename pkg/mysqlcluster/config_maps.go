package mysqlcluster

import (
	"bytes"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/go-ini/ini"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *cFactory) syncConfigMysqlMap() (state string, err error) {

	meta := metav1.ObjectMeta{
		Name: f.getNameForResource(ConfigMap),
		Labels: f.getLabels(map[string]string{
			"generated": "true"}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchConfigMap(f.client, meta,
		func(in *core.ConfigMap) *core.ConfigMap {
			data, err := f.getConfigMapData()
			if err != nil {
				glog.Errorf("Fail to create mysql configs. err: %s", err)
			}
			in.Data = data
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getConfigMapData() (map[string]string, error) {

	master, err := f.getMysqlConfigs(MysqlMasterConfigs)
	if err != nil {
		return nil, err
	}
	slave, err := f.getMysqlConfigs(MysqlSlaveConfigs)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"server-master-cnf": master,
		"server-slave-cnf":  slave,
	}, nil
}

func (f *cFactory) getMysqlConfigs(extraMysqld map[string]string) (string, error) {
	cfg := ini.Empty()
	s := cfg.Section("mysqld")

	for k, v := range extraMysqld {
		if _, err := s.NewKey(k, v); err != nil {
			return "", err
		}
	}

	for k, v := range MysqlMasterSlaveConfigs {
		if _, err := s.NewKey(k, v); err != nil {
			return "", err
		}
	}

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
