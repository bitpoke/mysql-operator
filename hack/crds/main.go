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
	"io"
	"os"
	"strings"

	kutil "github.com/appscode/kutil/apiextensions/v1beta1"
	"github.com/spf13/pflag"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/presslabs/mysql-operator/pkg/apis/mysql"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	myopenapi "github.com/presslabs/mysql-operator/pkg/openapi"
)

var (
	group   = mysql.GroupName
	version = "v1alpha1"
)

var crds = []kutil.Config{
	kutil.Config{
		Kind:       api.MysqlClusterKind,
		Plural:     api.MysqlClusterPlural,
		Singular:   "mysqlcluster",
		ShortNames: []string{"mysql", "mysql-cluster"},
	},
	kutil.Config{
		Kind:       api.MysqlBackupKind,
		Plural:     api.MysqlBackupPlural,
		Singular:   "mysqlbackup",
		ShortNames: []string{"backup", "mysql-backup"},
	},
}

func generateCRD(cfg kutil.Config, w io.Writer) error {
	fmt.Fprintf(os.Stderr, "Generating '%s' resource...\n", cfg.Kind)

	cfg.SpecDefinitionName = fmt.Sprintf("github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1.%s", cfg.Kind)
	cfg.EnableValidation = true
	cfg.ResourceScope = string(extensionsobj.NamespaceScoped)
	cfg.Group = group
	cfg.Version = version
	cfg.GetOpenAPIDefinitions = myopenapi.GetOpenAPIDefinitions

	crd := kutil.NewCustomResourceDefinition(cfg)
	kutil.MarshallCrd(w, crd, "yaml")
	return nil
}

func initFlags(cfg *kutil.Config, fs *pflag.FlagSet) *pflag.FlagSet {
	fs.Var(&cfg.Labels, "labels", "Lables")
	fs.Var(&cfg.Annotations, "annotations", "Annotations")
	return fs
}

func main() {
	kind := os.Args[1]
	for _, cfg := range crds {
		if strings.ToLower(cfg.Kind) == strings.ToLower(kind) {
			fs := pflag.NewFlagSet("test", pflag.ExitOnError)
			fs = initFlags(&cfg, fs)
			if err := fs.Parse(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to parse args: %s", err)
				return
			}
			err := generateCRD(cfg, os.Stdout)
			if err != nil {
				fmt.Printf("Fail to generate yaml for %s, err: %s\n", cfg.Kind, err)
			}

			return
		}
	}

	fmt.Fprintf(os.Stderr, "Kind '%s' not found!\n", kind)
}
