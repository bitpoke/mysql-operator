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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/presslabs/mysql-operator/pkg/apis/mysql"
)

var (
	// SchemeBuilder the scheme builder
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme function
	AddToScheme = SchemeBuilder.AddToScheme
	// SchemeGroupVersion ..
	SchemeGroupVersion = schema.GroupVersion{Group: mysql.GroupName, Version: "v1alpha1"}
)

// Resource gets an MysqlCluster GroupResource for a specified resource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&MysqlCluster{},
		&MysqlClusterList{},
		&MysqlBackup{},
		&MysqlBackupList{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}
