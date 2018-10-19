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
	"fmt"
	"strings"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const (
	// BackupSuffix is the file extension that will be uploaded into storage
	// provider
	BackupSuffix = "xbackup.gz"
)

var log = logf.Log.WithName("mysqlbackup")

// Wrapper is a type wrapper over MysqlBackup that contains the Business logic
type Wrapper struct {
	*api.MysqlBackup
}

// New returns a wraper object over MysqlBackup
func New(backup *api.MysqlBackup) *Wrapper {
	return &Wrapper{
		MysqlBackup: backup,
	}
}

// GetNameForJob returns the name of the job
func (w *Wrapper) GetNameForJob() string {
	return fmt.Sprintf("%s-bjob", w.Name)
}

// GetBackupURL returns a backup URL
func (w *Wrapper) GetBackupURL(cluster *api.MysqlCluster) string {
	if strings.HasSuffix(w.Spec.BackupURL, BackupSuffix) {
		return w.Spec.BackupURL
	}

	if len(w.Spec.BackupURL) > 0 {
		return w.composeBackupURL(w.Spec.BackupURL)
	}

	if len(cluster.Spec.BackupURL) == 0 {
		cluster.Spec.BackupURL = cluster.Spec.BackupURI
	}

	if len(cluster.Spec.BackupURL) == 0 {
		return ""
	}
	return w.composeBackupURL(cluster.Spec.BackupURL)
}

func (w *Wrapper) composeBackupURL(base string) string {
	if strings.HasSuffix(base, "/") {
		base = base[:len(base)-1]
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05")
	fileName := fmt.Sprintf("/%s-%s.%s", w.Spec.ClusterName, timestamp, BackupSuffix)
	return base + fileName
}
