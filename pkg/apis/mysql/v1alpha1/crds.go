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

package v1alpha1

import (
	"fmt"

	kutil "github.com/appscode/kutil/apiextensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	myopenapi "github.com/presslabs/mysql-operator/pkg/openapi"
)

const (
	myapiPkg                 = "github.com/presslabs/mysql-operator/pkg/apis/mysql"
	ResourceKindMysqlCluster = "MysqlCluster"
	ResourceKindMysqlBackup  = "MysqlBackup"
)

// Mysql Operator Custom Resource Definition
var (
	// ResourceMysqlCluster contains the definition bits for Mysql Cluster CRD
	ResourceMysqlCluster = kutil.Config{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,

		Kind:       ResourceKindMysqlCluster,
		Plural:     "mysqlclusters",
		Singular:   "mysqlcluster",
		ShortNames: []string{"mysql", "cluster"},

		SpecDefinitionName:    fmt.Sprintf("%s/%s.%s", myapiPkg, SchemeGroupVersion.Version, ResourceKindMysqlCluster),
		ResourceScope:         string(apiextensions.NamespaceScoped),
		GetOpenAPIDefinitions: myopenapi.GetOpenAPIDefinitions,

		EnableValidation:        true,
		EnableStatusSubresource: true,
	}
	// ResourceMysqlClusterCRDName is the fully qualified MysqlCluster CRD name (ie. mysqlclusters.mysql.presslabs.org)
	ResourceMysqlClusterCRDName = fmt.Sprintf("%s.%s", ResourceMysqlCluster.Plural, ResourceMysqlCluster.Group)
	// ResourceMysqlClusterCRD is the Custrom Resource Definition object for MysqlCluster
	ResourceMysqlClusterCRD = kutil.NewCustomResourceDefinition(ResourceMysqlCluster)

	ResourceMysqlBackup = kutil.Config{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,

		Kind:       ResourceKindMysqlBackup,
		Plural:     "mysqlbackups",
		Singular:   "mysqlbackup",
		ShortNames: []string{"backup"},

		SpecDefinitionName:    fmt.Sprintf("%s/%s.%s", myapiPkg, SchemeGroupVersion.Version, ResourceKindMysqlBackup),
		ResourceScope:         string(apiextensions.NamespaceScoped),
		GetOpenAPIDefinitions: myopenapi.GetOpenAPIDefinitions,

		EnableValidation:        true,
		EnableStatusSubresource: true,
	}
	// ResourceMysqlBackupCRDName is the fully qualified MysqlBackup CRD name (ie. mysqlbackups.mysql.presslabs.org)
	ResourceMysqlBackupCRDName = fmt.Sprintf("%s.%s", ResourceMysqlBackup.Plural, ResourceMysqlBackup.Group)
	// ResourceMysqlBackupCRD is the Custrom Resource Definition object for MysqlBackup
	ResourceMysqlBackupCRD = kutil.NewCustomResourceDefinition(ResourceMysqlBackup)
)

var CRDs = map[string]kutil.Config{
	ResourceMysqlClusterCRDName: ResourceMysqlCluster,
	ResourceMysqlBackupCRDName:  ResourceMysqlBackup,
}
