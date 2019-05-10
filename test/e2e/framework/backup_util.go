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

package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func GetBucketName() string {
	bucket := os.Getenv("BACKUP_BUCKET_NAME")
	if len(bucket) == 0 {
		Logf("BACKUP_BUEKET_NAME not set! Backups tests will not work")
	}
	return fmt.Sprintf("gs://%s", bucket)
}

func (f *Framework) NewGCSBackupSecret() *corev1.Secret {
	json_key := os.Getenv("GOOGLE_CREDENTIALS")
	if json_key == "" {
		Logf("GOOGLE_CREDENTIALS is not set! Backups tests will not work")
	}

	jk := make(map[string]string)
	if err := json.Unmarshal([]byte(json_key), &jk); err != nil {
		panic("Failed to unmarshal GOOGLE_CREDENTIALS env value")
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("backup-secret-%s", f.BaseName),
			Namespace: f.Namespace.Name,
		},
		StringData: map[string]string{
			"GCS_SERVICE_ACCOUNT_JSON_KEY": json_key,
			"GCS_PROJECT_ID":               jk["project_id"],
		},
	}
}

func NewBackup(cluster *api.MysqlCluster, bucket string) *api.MysqlBackup {
	return &api.MysqlBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bk-%s", cluster.Name),
			Namespace: cluster.Namespace,
		},
		Spec: api.MysqlBackupSpec{
			ClusterName: cluster.Name,
			BackupURL:   fmt.Sprintf("%s/%s", bucket, cluster.Name),
			// the secret is specified on the cluster, no need to specify it here.
		},
	}
}

func (f *Framework) RefreshBackupFn(backup *api.MysqlBackup) func() *api.MysqlBackup {
	return func() *api.MysqlBackup {
		key := types.NamespacedName{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		}
		b := &api.MysqlBackup{}
		f.Client.Get(context.TODO(), key, b)
		return b
	}
}

// BackupCompleted a matcher to check cluster completion
func BackupCompleted() gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Completed": Equal(true),
		}),
	}))
}

// HaveBackupCond is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
func HaveBackupCond(condType api.BackupConditionType, status corev1.ConditionStatus) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(condType),
				"Status": Equal(status),
			})),
		})},
	))
}

// GetNameForJob returns the job name of a backup
func GetNameForJob(backup *api.MysqlBackup) string {
	return fmt.Sprintf("%s-backup", backup.Name)
}
