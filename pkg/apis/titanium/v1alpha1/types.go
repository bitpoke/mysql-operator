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
	Replicas   int32  `json:"replicas"`
	SecretName string `json:secretName`

	MysqlRootPassword string `json:mysqlRootPassword`

	MysqlReplicationUser     string `json:mysqlReplicationUser,omitempty`
	MysqlReplicationPassword string `json:mysqlReplicationPassword,omitempty`

	MysqlUser     string `json:mysqlUser,omitempty`
	MysqlPassword string `json:mysqlPassword,omitempty`
	MysqlDatabase string `json:mysqlDatabase,omitempty`

	InitBucketURI        string `json:initBucketURI,omitempty`
	InitBucketSecretName string `json:initBucketSecretName,omitempty`

	BackupBucketURI        string `json:backupBucketURI,omitempty`
	BackupBucketSecertName string `json:backupBucketSecretName,omitempty`

	BackupSchedule string `json:backupSchedule,omitempty`

	PodSpec     PodSpec     `json:podSpec,omitempty`
	MysqlConfig MysqlConfig `json:mysqlConfig,omitempty`
	VolumeSpec  VolumeSpec  `json:volumeSpec,omitempty`
}

type MysqlConfig map[string]string

type ClusterStatus struct {
	Conditions []ClusterCondition `json:conditions`
}

type ClusterCondition struct {
	// type of cluster condition, values in (\"Ready\")
	Type ClusterConditionType `json:type`
	// Status of the condition, one of (\"True\", \"False\", \"Unknown\")
	Status ConditionStatus `json:status`

	// optional TODO: detail each member.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Reason             string      `json:"reason"`
	Message            string      `json:"message"`
}

type ClusterConditionType string

const (
	ClusterConditionReady ClusterConditionType = "Ready"
)

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type PodSpec struct {
	Image                   string                       `json:image,omitempty`
	ImagePullPolicy         apiv1.PullPolicy             `json:imagePullPolicy,omitempty`
	TitaniumImage           string                       `json:titaniumImage,omitempty`
	TitaniumImagePullPolicy apiv1.PullPolicy             `json:titaniumImagePullPolicy,omitempty`
	MetricsImage            string                       `json:metricsImage,omitempty`
	MetricsImagePullPolicy  apiv1.PullPolicy             `json:metricsImagePullPolicy,omitempty`
	ImagePullSecrets        []apiv1.LocalObjectReference `json:imagePullSecrets,omitempty`

	Labels       map[string]string          `json:labels`
	Annotations  map[string]string          `json:annotations`
	Resources    apiv1.ResourceRequirements `json:resources`
	Affinity     apiv1.Affinity             `json:affinity`
	NodeSelector map[string]string          `json:nodeSelector`
}

type VolumeSpec struct {
	apiv1.PersistentVolumeClaimSpec `json:",inline"`

	PersistenceDisabled bool `json:persistenceDisabled,omitempty`
}
