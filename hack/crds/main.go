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

package main

import (
	"fmt"
	"os"

	kutil "github.com/appscode/kutil/apiextensions/v1beta1"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

var (
	group           = "mysql.presslabs.net"
	version         = "v1alpha1"
	clusterYamlPath = "hack/crd-cluster.yaml"
	backupYamlPath  = "hack/crd-backup.yaml"
)

func generateCRD(cfg kutil.Config, path string) error {
	fmt.Printf("Generateing %s resource at %s\n", cfg.Kind, path)

	cfg.SpecDefinitionName = fmt.Sprintf("github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1.%s", cfg.Kind)
	cfg.EnableValidation = true
	cfg.ResourceScope = string(extensionsobj.NamespaceScoped)
	cfg.Group = group
	cfg.Version = version
	cfg.GetOpenAPIDefinitions = api.GetOpenAPIDefinitions

	crd := kutil.NewCustomResourceDefinition(cfg)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Fail to create file: %s\n", path)
	}
	defer f.Close()

	kutil.MarshallCrd(f, crd, "yaml")
	return nil
}

func main() {
	err := generateCRD(kutil.Config{
		Kind:       api.MysqlClusterKind,
		Plural:     api.MysqlClusterPlural,
		Singular:   "mysqlcluster",
		ShortNames: []string{"mysql-cluster"},
	}, clusterYamlPath)
	if err != nil {
		fmt.Errorf("Fail to generate yaml for mysql cluster, err: %s", err)
	}

	err = generateCRD(kutil.Config{
		Kind:       api.MysqlBackupKind,
		Plural:     api.MysqlBackupPlural,
		Singular:   "mysqlbackup",
		ShortNames: []string{"mysql-backup"},
	}, backupYamlPath)
	if err != nil {
		fmt.Errorf("Fail to generate yaml for mysql backup, err: %s", err)
	}
}
