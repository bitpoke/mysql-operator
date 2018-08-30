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

	"github.com/go-ini/ini"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/syncer"
)

type configMapSyncer struct {
	cluster *api.MysqlCluster
}

// NewConfigMapSyncer returns config map syncer
func NewConfigMapSyncer(cluster *api.MysqlCluster) syncer.Interface {
	return &configMapSyncer{
		cluster: cluster,
	}
}

func (s *configMapSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.cluster.GetNameForResource(api.ConfigMap),
			Namespace: s.cluster.Namespace,
		},
	}
}

func (s *configMapSyncer) ShouldHaveOwnerReference() bool {
	return true
}

func (s *configMapSyncer) Sync(in runtime.Object) error {
	out := in.(*core.ConfigMap)

	out.ObjectMeta.Labels = s.cluster.GetLabels()
	out.ObjectMeta.Labels["generated"] = "true"

	data, newHash, err := s.getMysqlConfData()
	if err != nil {
		return fmt.Errorf("failed to create mysql configs, err: %s", err)
	}

	if key, ok := out.ObjectMeta.Annotations["config_hash"]; ok {
		if key == newHash {
			glog.V(2).Infof("Skip updating configs, it's up to date: %s",
				out.ObjectMeta.Annotations["config_hash"])
			return nil
		}
		glog.V(2).Infof("Config map hashes doesn't match: %s != %s. Updateing config map.", key, newHash)
	}

	out.ObjectMeta.Annotations = map[string]string{
		"config_hash": newHash,
	}

	out.Data = map[string]string{
		"my.cnf": data,
	}

	return nil
}

func (s *configMapSyncer) getMysqlConfData() (string, string, error) {
	cfg := ini.Empty()
	sec := cfg.Section("mysqld")

	addKVConfigsToSection(sec, api.MysqlMasterSlaveConfigs, s.cluster.Spec.MysqlConf)

	// include configs from /etc/mysql/conf.d/*.cnf
	_, err := sec.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))
	if err != nil {
		return "", "", err
	}

	data, err := writeConfigs(cfg)
	if err != nil {
		return "", "", err
	}
	hash, err := hashstructure.Hash(cfg.Section("mysqld").KeysHash(), nil)
	if err != nil {
		glog.Errorf("Can't compute hash for map data: %s", err)
		return "", "", err
	}

	newHash := strconv.FormatUint(hash, 10)
	return data, newHash, nil

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

// helper function to write to string ini.File
// nolint: interfacer
func writeConfigs(cfg *ini.File) (string, error) {
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
