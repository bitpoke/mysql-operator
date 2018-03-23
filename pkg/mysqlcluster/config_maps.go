package mysqlcluster

import (
	"bytes"
	"fmt"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/go-ini/ini"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
)

func (f *cFactory) syncConfigMysqlMap() (state string, err error) {

	meta := metav1.ObjectMeta{
		Name: f.cluster.GetNameForResource(api.ConfigMap),
		Labels: f.getLabels(map[string]string{
			"generated": "true"}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kcore.CreateOrPatchConfigMap(f.client, meta,
		func(in *core.ConfigMap) *core.ConfigMap {
			if key := in.ObjectMeta.Annotations["config_version"]; key == ConfigVersion {
				glog.V(2).Infof("Skip updating configs, it's up to date: %s",
					in.ObjectMeta.Annotations["config_version"])
				return in
			}
			in.ObjectMeta.Annotations = map[string]string{
				"config_version": ConfigVersion,
			}
			data, err := f.getConfigMapData()
			if err != nil {
				glog.Errorf("Fail to create mysql configs. err: %s", err)
				return in
			}
			in.Data = data
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getConfigMapData() (map[string]string, error) {
	cnf, err := f.getMysqlConfigs(MysqlMasterSlaveConfigs, f.cluster.Spec.MysqlConf)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"my.cnf": cnf,
	}, nil
}

func (f *cFactory) getMysqlConfigs(extraMysqld ...map[string]string) (string, error) {
	cfg := ini.Empty()
	s := cfg.Section("mysqld")

	for _, extra := range extraMysqld {
		for k, v := range extra {
			if _, err := s.NewKey(k, v); err != nil {
				return "", err
			}
		}
	}

	// include configs from /etc/mysql/conf.d/*.cnf
	s.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
