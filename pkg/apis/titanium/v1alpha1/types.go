/*
Copyright 2017 The Kubernetes Authors.
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
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=mysqlcluster

type MysqlCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterSpec   `json:"spec"`
	Status            ClusterStatus `json:status,omitempty`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MysqlClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlCluster `json:"items"`
}

type ClusterSpec struct {
	// The number of slaves pods. This updates replicas filed
	// Defaults to 0
	// +optional
	Replicas int32 `json:"replicas"`
	// The secret name that contains connection information to initialize database, like
	// USER, PASSWORD, ROOT_PASSWORD and so on
	// This secret will be updated with DB_CONNECT_URL and some more configs.
	// Can be specified partially
	// Defaults is <name>-db-credentials (with random values)
	// +optional
	SecretName string `json:"secretName"`

	// Represents the percona image tag.
	// Defaults to 5.7
	// +optional
	MysqlVersion string `json:"mysqlVersion"`

	// A bucket URI that contains a xtrabackup to initialize the mysql database.
	// +optional
	InitBucketURI        string `json:initBucketURI,omitempty`
	InitBucketSecretName string `json:initBucketSecretName,omitempty`

	// A bucket URI to save backups.
	// +optional
	BackupBucketURI        string `json:backupBucketURI,omitempty`
	BackupBucketSecretName string `json:backupBucketSecretName,omitempty`

	// Specify under crontab format interval to take backups
	// leave it empty to deactivate the backup process
	// Defaults to ""
	// +optional
	BackupSchedule string `json:backupSchedule,omitempty`

	// A map[string]string that will be passed to my.cnf file.
	// +optional
	MysqlConf MysqlConf `json:mysqlConf,omitempty`

	// Pod extra specification
	// +optional
	PodSpec PodSpec `json:podSpec,omitempty`

	// PVC extra specifiaction
	// +optional
	VolumeSpec `json:volumeSpec,omitempty`
}

type MysqlConf map[string]string

type ClusterStatus struct {
	// ReadyNodes represents number of the nodes that are in ready state
	ReadyNodes int
	// Conditions contains the list of the cluster conditions fulfilled
	Conditions []ClusterCondition `json:conditions`
}

type ClusterCondition struct {
	// type of cluster condition, values in (\"Ready\")
	Type ClusterConditionType `json:type`
	// Status of the condition, one of (\"True\", \"False\", \"Unknown\")
	Status apiv1.ConditionStatus `json:status`

	// optional TODO: detail each member.
	// are optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Reason             string      `json:"reason"`
	Message            string      `json:"message"`
}

type ClusterConditionType string

const (
	ClusterConditionReady        ClusterConditionType = "Ready"
	ClusterConditionInitDefaults ClusterConditionType = "InitDefaults"

	ClusterConditionConfig ClusterConditionType = "ConfigReady"
)

type PodSpec struct {
	ImagePullPolicy  apiv1.PullPolicy             `json:imagePullPolicy,omitempty`
	ImagePullSecrets []apiv1.LocalObjectReference `json:imagePullSecrets,omitempty`

	Labels       map[string]string          `json:labels`
	Annotations  map[string]string          `json:annotations`
	Resources    apiv1.ResourceRequirements `json:resources`
	Affinity     apiv1.Affinity             `json:affinity`
	NodeSelector map[string]string          `json:nodeSelector`
}

type VolumeSpec struct {
	apiv1.PersistentVolumeClaimSpec `json:",inline"`
}
