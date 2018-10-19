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

package mysqlbackup

import (
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// SetDefaults sets default for backup
func (w *Wrapper) SetDefaults(cluster *api.MysqlCluster) {
	// the source of truth is BackupURL if this is not set then use what is in
	// BackupURI
	if len(w.Spec.BackupURL) == 0 {
		w.Spec.BackupURL = w.Spec.BackupURI
	}

	w.Spec.BackupURL = w.GetBackupURL(cluster)

	if len(w.Spec.BackupSecretName) == 0 {
		w.Spec.BackupSecretName = cluster.Spec.BackupSecretName
	}
}
