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
			data, hash, err := f.getConfigMapData()
			if err != nil {
				glog.Errorf("Fail to create mysql configs. err: %s", err)
				return in
			}
			current_hash := strconv.FormatUint(hash, 10)
			f.configHash = current_hash

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

			in.Data = data
			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getConfigMapData() (map[string]string, uint64, error) {
	cnf, hash, err := f.getMysqlConfigs(MysqlMasterSlaveConfigs, f.cluster.Spec.MysqlConf)
	if err != nil {
		return nil, hash, err
	}
	return map[string]string{
		"my.cnf": cnf,
	}, hash, nil
}

func (f *cFactory) getMysqlConfigs(extraMysqld ...map[string]string) (string, uint64, error) {
	cfg := ini.Empty()
	s := cfg.Section("mysqld")

	for _, extra := range extraMysqld {
		for k, v := range extra {
			if _, err := s.NewKey(k, v); err != nil {
				return "", 0, err
			}
		}
	}

	hash, err := hashstructure.Hash(extraMysqld, nil)
	if err != nil {
		glog.Errorf("Can't compute hash for map data: %s", err)
		return "", 0, err
	}

	// include configs from /etc/mysql/conf.d/*.cnf
	s.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", 0, err
	}
	return buf.String(), hash, nil
}
