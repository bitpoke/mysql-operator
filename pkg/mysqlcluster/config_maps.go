/*
Copyright 2018 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
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
			data, new_hash, err := f.getMysqlConfData()
			if err != nil {
				glog.Errorf("Fail to create mysql configs. err: %s", err)
				return in
			}

			f.configHash = new_hash

			if key, ok := in.ObjectMeta.Annotations["config_hash"]; ok {
				if key == new_hash {
					glog.V(2).Infof("Skip updating configs, it's up to date: %s",
						in.ObjectMeta.Annotations["config_hash"])
					return in
				} else {
					glog.Infof("Config hashes doesn't match: %s != %s . Updateing configs.", key, new_hash)
				}
			}

			in.ObjectMeta.Annotations = map[string]string{
				"config_hash": new_hash,
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

	data, err := writeConfigs(cfg)
	if err != nil {
		return "", "", err
	}
	hash, err := hashstructure.Hash(cfg.Section("mysqld").KeysHash(), nil)
	if err != nil {
		glog.Errorf("Can't compute hash for map data: %s", err)
		return "", "", err
	}

	new_hash := strconv.FormatUint(hash, 10)
	return data, new_hash, nil

}

// helper function to add a map[string]string to a ini.Section
func addKVConfigsToSection(s *ini.Section, extraMysqld ...map[string]string) {
	for _, extra := range extraMysqld {
		for k, v := range extra {
			if _, err := s.NewKey(k, v); err != nil {
				glog.Errorf("Failed to add '%s':'%s' to config section, err: %s", k, v, err)
			}
		}
	}
}

// helper function to add a string to a ini.Section
func addBConfigsToSection(s *ini.Section, boolConfigs ...[]string) {
	for _, extra := range boolConfigs {
		for _, k := range extra {
			if _, err := s.NewBooleanKey(k); err != nil {
				glog.Errorf("Failed to add boolean key '%s' to config section, err: %s", k, err)
			}
		}
	}
}

// helper function to write to string ini.File
func writeConfigs(cfg *ini.File) (string, error) {
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
