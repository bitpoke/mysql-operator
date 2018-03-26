package mysqlcluster

import (
	"bytes"
	"fmt"
	"strconv"

	kcore "github.com/appscode/kutil/core/v1"
	"github.com/go-ini/ini"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure"
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
			data, current_hash, err := f.getMysqlConfData()
			if err != nil {
				glog.Errorf("Fail to create mysql configs. err: %s", err)
				return in
			}

			if key, ok := in.ObjectMeta.Annotations["config_hash"]; ok {
				if key == current_hash {
					glog.V(2).Infof("Skip updating configs, it's up to date: %s",
						in.ObjectMeta.Annotations["config_hash"])
					return in
				} else {
					glog.Infof("Config hashes doesn't match: %s != %s . Updateing configs.", key, current_hash)
				}
			}
			in.ObjectMeta.Annotations = map[string]string{
				"config_hash": current_hash,
			}

			in.Data = map[string]string{
				"my.cnf": data,
			}

			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getMysqlConfData() (string, string, error) {
	cfg := ini.Empty()
	s := cfg.Section("mysqld")

	addKVConfigsToSection(s, MysqlMasterSlaveConfigs, f.cluster.Spec.MysqlConf)
	addBConfigsToSection(s, MysqlMasterSlaveBooleanConfigs)

	// include configs from /etc/mysql/conf.d/*.cnf
	s.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))

	data, hash, err := writeConfigs(cfg)
	if err != nil {
		return "", "", err
	}

	current_hash := strconv.FormatUint(hash, 10)
	f.configHash = current_hash

	return data, current_hash, nil

}

func addKVConfigsToSection(s *ini.Section, extraMysqld ...map[string]string) {
	for _, extra := range extraMysqld {
		for k, v := range extra {
			if _, err := s.NewKey(k, v); err != nil {
				glog.Errorf("Failed to add '%s':'%s' to config section, err: %s", k, v, err)
			}
		}
	}
}

func addBConfigsToSection(s *ini.Section, boolConfigs ...[]string) {
	for _, extra := range boolConfigs {
		for _, k := range extra {
			if _, err := s.NewBooleanKey(k); err != nil {
				glog.Errorf("Failed to add boolean key '%s' to config section, err: %s", k, err)
			}
		}
	}
}

func writeConfigs(cfg *ini.File) (string, uint64, error) {
	hash, err := hashstructure.Hash(cfg.Section("mysqld").KeysHash(), nil)
	if err != nil {
		glog.Errorf("Can't compute hash for map data: %s", err)
		return "", 0, err
	}

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", 0, err
	}
	return buf.String(), hash, nil
}
