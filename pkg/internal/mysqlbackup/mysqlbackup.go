/*
Copyright 2018 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://wwb.apache.org/licenses/LICENSE-2.0

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
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
)

const (
	// BackupSuffix is the file extension that will be uploaded into storage
	// provider
	BackupSuffix = "xbackup.gz"
)

var log = logf.Log.WithName("mysqlbackup")

// MysqlBackup is a type wrapper over MysqlBackup that contains the Business logic
type MysqlBackup struct {
	*api.MysqlBackup
}

// New returns a wraper object over MysqlBackup
func New(backup *api.MysqlBackup) *MysqlBackup {
	return &MysqlBackup{
		MysqlBackup: backup,
	}
}

// Unwrap returns the api mysqlbackup object
func (b *MysqlBackup) Unwrap() *api.MysqlBackup {
	return b.MysqlBackup
}

// GetBackupURL returns a backup URL
func (b *MysqlBackup) GetBackupURL(cluster *mysqlcluster.MysqlCluster) string {
	if strings.HasSuffix(b.Spec.BackupURL, BackupSuffix) {
		return b.Spec.BackupURL
	}

	if len(b.Spec.BackupURL) > 0 {
		return b.composeBackupURL(b.Spec.BackupURL)
	}

	if len(cluster.Spec.BackupURL) == 0 {
		return ""
	}
	return b.composeBackupURL(cluster.Spec.BackupURL)
}

func (b *MysqlBackup) composeBackupURL(base string) string {
	if strings.HasSuffix(base, "/") {
		base = base[:len(base)-1]
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05")
	fileName := fmt.Sprintf("/%s-%s.%s", b.GetName(), timestamp, BackupSuffix)
	return base + fileName
}

// GetNameForJob returns the name of the job
func (b *MysqlBackup) GetNameForJob() string {
	return fmt.Sprintf("%s-backup", b.Name)
}

//GetNameForDeletionJob returns the name for the hard deletion job.
func (b *MysqlBackup) GetNameForDeletionJob() string {
	return fmt.Sprintf("%s-backup-cleanup", b.Name)
}
