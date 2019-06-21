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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be
// serialized.

// MysqlBackupSpec defines the desired state of MysqlBackup
type MysqlBackupSpec struct {
	// ClustterName represents the cluster for which to take backup
	ClusterName string `json:"clusterName"`

	// BackupURL represents the URL to the backup location, this can be
	// partially specifyied. Default is used the one specified in the cluster.
	// +optional
	BackupURL string `json:"backupURL,omitempty"`

	// BackupSecretName the name of secrets that contains the credentials to
	// access the bucket. Default is used the secret specified in cluster.
	// +optional
	BackupSecretName string `json:"backupSecretName,omitempty"`

	// RemoteDeletePolicy the deletion policy that specify how to treat the data from remote storage. By
	// default it's used softDelete.
	// +optional
	RemoteDeletePolicy DeletePolicy `json:"remoteDeletePolicy,omitempty"`
}

// BackupCondition defines condition struct for backup resource
type BackupCondition struct {
	// type of cluster condition, values in (\"Ready\")
	Type BackupConditionType `json:"type"`
	// Status of the condition, one of (\"True\", \"False\", \"Unknown\")
	Status core.ConditionStatus `json:"status"`

	// LastTransitionTime
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason
	Reason string `json:"reason"`
	// Message
	Message string `json:"message"`
}

// BackupConditionType defines condition types of a backup resources
type BackupConditionType string

const (
	// BackupComplete means the backup has finished his execution
	BackupComplete BackupConditionType = "Complete"
	// BackupFailed means backup has failed
	BackupFailed BackupConditionType = "Failed"
)

// DeletePolicy defines the types of policies for backup deletions are
type DeletePolicy string

const (
	// Delete when used it will try to delete the backup from remote storage then will remove the
	// MysqlBackup resource from Kubernetes. The remote deletion is not guaranteed that will succeed.
	Delete DeletePolicy = "delete"
	// Retain when used it will delete only the MysqlBackup resource from Kuberentes and will keep the backup
	// on remote storage.
	Retain DeletePolicy = "retain"
)

// MysqlBackupStatus defines the observed state of MysqlBackup
type MysqlBackupStatus struct {
	// Completed indicates whether the backup is in a final state,
	// no matter whether its' corresponding job failed or succeeded
	Completed bool `json:"completed,omitempty"`
	// Conditions represents the backup resource conditions list.
	Conditions []BackupCondition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlBackup is the Schema for the mysqlbackups API
// +k8s:openapi-gen=true
type MysqlBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlBackupSpec   `json:"spec,omitempty"`
	Status MysqlBackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlBackupList contains a list of MysqlBackup
type MysqlBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlBackup{}, &MysqlBackupList{})
}
