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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
// Important: Run "make" to regenerate code after modifying this file

// MysqlClusterSpec defines the desired state of MysqlCluster
// nolint: maligned
type MysqlClusterSpec struct {
	// The number of pods. This updates replicas filed
	// Defaults to 0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// The secret name that contains connection information to initialize database, like
	// USER, PASSWORD, ROOT_PASSWORD and so on
	// This secret will be updated with DB_CONNECT_URL and some more configs.
	// Can be specified partially
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	SecretName string `json:"secretName"`

	// Represents the MySQL version that will be run. The available version can be found here:
	// https://github.com/presslabs/mysql-operator/blob/0fd4641ce4f756a0aab9d31c8b1f1c44ee10fcb2/pkg/util/constants/constants.go#L87
	// This field should be set even if the Image is set to let the operator know which mysql version is running.
	// Based on this version the operator can take decisions which features can be used.
	// Defaults to 5.7
	// +optional
	MysqlVersion string `json:"mysqlVersion,omitempty"`

	// To specify the image that will be used for mysql server container.
	// If this is specified then the mysqlVersion is used as source for MySQL server version.
	// +optional
	Image string `json:"image,omitempty"`

	// A bucket URL that contains a xtrabackup to initialize the mysql database.
	// +optional
	InitBucketURL string `json:"initBucketURL,omitempty"`
	// Same as InitBucketURL but is DEPRECATED
	// +optional
	InitBucketURI        string `json:"initBucketURI,omitempty"`
	InitBucketSecretName string `json:"initBucketSecretName,omitempty"`

	// The number of pods from that set that must still be available after the
	// eviction, even in the absence of the evicted pod
	// Defaults to 50%
	// +optional
	MinAvailable string `json:"minAvailable,omitempty"`

	// Specify under crontab format interval to take backups
	// leave it empty to deactivate the backup process
	// Defaults to ""
	// +optional
	BackupSchedule string `json:"backupSchedule,omitempty"`

	// Represents an URL to the location where to put backups.
	// +optional
	BackupURL string `json:"backupURL,omitempty"`

	// BackupRemoteDeletePolicy the deletion policy that specify how to treat the data from remote storage. By
	// default it's used softDelete.
	// +optional
	BackupRemoteDeletePolicy DeletePolicy `json:"backupRemoteDeletePolicy,omitempty"`

	// Represents the name of the secret that contains credentials to connect to
	// the storage provider to store backups.
	// +optional
	BackupSecretName string `json:"backupSecretName,omitempty"`

	// If set keeps last BackupScheduleJobsHistoryLimit Backups
	// +optional
	BackupScheduleJobsHistoryLimit *int `json:"backupScheduleJobsHistoryLimit,omitempty"`

	// A map[string]string that will be passed to my.cnf file.
	// +optional
	MysqlConf MysqlConf `json:"mysqlConf,omitempty"`

	// Pod extra specification
	// +optional
	PodSpec PodSpec `json:"podSpec,omitempty"`

	// PVC extra specifiaction
	// +optional
	VolumeSpec VolumeSpec `json:"volumeSpec,omitempty"`

	// TmpfsSize if specified, mounts a tmpfs of this size into /tmp
	// +optional
	TmpfsSize string `json:"tmpfsSize,omitempty"`

	// MaxSlaveLatency represents the allowed latency for a slave node in
	// seconds. If set then the node with a latency grater than this is removed
	// from service.
	// +optional
	MaxSlaveLatency *int64 `json:"maxSlaveLatency,omitempty"`

	// QueryLimits represents limits for a query
	// +optional
	QueryLimits *QueryLimits `json:"queryLimits,omitempty"`

	// Makes the cluster READ ONLY. Set the master to writable or ReadOnly
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`

	// Set a custom offset for Server IDs.  ServerID for each node will be the index of the statefulset, plus offset
	// +optional
	ServerIDOffset *int `json:"serverIDOffset,omitempty"`
}

// MysqlConf defines type for extra cluster configs. It's a simple map between
// string and string.
type MysqlConf map[string]intstr.IntOrString

// PodSpec defines type for configure cluster pod spec.
type PodSpec struct {
	ImagePullPolicy  core.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []core.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Labels             map[string]string         `json:"labels,omitempty"`
	Annotations        map[string]string         `json:"annotations,omitempty"`
	Resources          core.ResourceRequirements `json:"resources,omitempty"`
	Affinity           *core.Affinity            `json:"affinity,omitempty"`
	NodeSelector       map[string]string         `json:"nodeSelector,omitempty"`
	PriorityClassName  string                    `json:"priorityClassName,omitempty"`
	Tolerations        []core.Toleration         `json:"tolerations,omitempty"`
	ServiceAccountName string                    `json:"serviceAccountName,omitempty"`
}

// VolumeSpec is the desired spec for storing mysql data. Only one of its
// members may be specified.
type VolumeSpec struct {
	// EmptyDir to use as data volume for mysql. EmptyDir represents a temporary
	// directory that shares a pod's lifetime.
	// +optional
	EmptyDir *core.EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// HostPath to use as data volume for mysql. HostPath represents a
	// pre-existing file or directory on the host machine that is directly
	// exposed to the container.
	// +optional
	HostPath *core.HostPathVolumeSource `json:"hostPath,omitempty"`

	// PersistentVolumeClaim to specify PVC spec for the volume for mysql data.
	// It has the highest level of precedence, followed by HostPath and
	// EmptyDir. And represents the PVC specification.
	// +optional
	PersistentVolumeClaim *core.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

// QueryLimits represents the pt-kill parameters, more info can be found
// here: https://www.percona.com/doc/percona-toolkit/LATEST/pt-kill.html
type QueryLimits struct {
	// MaxIdleTime match queries that have been idle for longer then this time,
	// in seconds. (--idle-time flag)
	// + optional
	MaxIdleTime *int `json:"maxIdleTime,omitempty"`

	// MaxQueryTime match queries that have been running for longer then this
	// time, in seconds. This field is required. (--busy-time flag)
	MaxQueryTime int `json:"maxQueryTime"`

	// Kill represents the mode of which the matching queries in each class will
	// be killed, (the --victims flag). Can be one of oldest|all|all-but-oldest.
	// By default, the matching query with the highest Time value is killed (the
	// oldest query.
	// +optional
	Kill string `json:"kill,omitempty"`

	// KillMode can be: `connection` or `query`, when it's used `connection`
	// means that when a query is matched the connection is killed (using --kill
	// flag) and if it's used `query` means that the query is killed (using
	// --kill-query flag)
	// +optional
	KillMode string `json:"killMode,omitempty"`

	// IgnoreDb is the list of database that are ignored by pt-kill (--ignore-db
	// flag).
	// +optional
	IgnoreDb []string `json:"ignoreDb,omitempty"`

	// IgnoreCommands the list of commands to be ignored.
	// +optional
	IgnoreCommand []string `json:"ignoreCommands,omitempty"`

	// IgnoreUser the list of users to be ignored.
	// +optional
	IgnoreUser []string `json:"ignoreUser,omitempty"`
}

// ClusterCondition defines type for cluster conditions.
type ClusterCondition struct {
	// type of cluster condition, values in (\"Ready\")
	Type ClusterConditionType `json:"type"`
	// Status of the condition, one of (\"True\", \"False\", \"Unknown\")
	Status core.ConditionStatus `json:"status"`

	// LastTransitionTime
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason
	Reason string `json:"reason"`
	// Message
	Message string `json:"message"`
}

// ClusterConditionType defines type for cluster condition type.
type ClusterConditionType string

const (
	// ClusterConditionReady represents the readiness of the cluster. This
	// condition is the same sa statefulset Ready condition.
	ClusterConditionReady ClusterConditionType = "Ready"

	// ClusterConditionFailoverAck represents if the cluster has pending ack in
	// orchestrator or not.
	ClusterConditionFailoverAck ClusterConditionType = "PendingFailoverAck"

	// ClusterConditionReadOnly describe cluster state if it's in read only or
	// writable.
	ClusterConditionReadOnly ClusterConditionType = "ReadOnly"

	// ClusterConditionFailoverInProgress indicates if there is a current failover in progress
	// done by the Orchestrator
	ClusterConditionFailoverInProgress ClusterConditionType = "FailoverInProgress"
)

// NodeStatus defines type for status of a node into cluster.
type NodeStatus struct {
	Name       string          `json:"name"`
	Conditions []NodeCondition `json:"conditions,omitempty"`
}

// NodeCondition defines type for representing node conditions.
type NodeCondition struct {
	Type               NodeConditionType    `json:"type"`
	Status             core.ConditionStatus `json:"status"`
	LastTransitionTime metav1.Time          `json:"lastTransitionTime"`
}

// NodeConditionType defines type for node condition type.
type NodeConditionType string

const (
	// NodeConditionLagged represents if the node is marked as lagged by
	// orchestrator.
	NodeConditionLagged NodeConditionType = "Lagged"
	// NodeConditionReplicating represents if the node is replicating or not.
	NodeConditionReplicating NodeConditionType = "Replicating"
	// NodeConditionMaster represents if the node is master or not.
	NodeConditionMaster NodeConditionType = "Master"
	// NodeConditionReadOnly repesents if the node is read only or not
	NodeConditionReadOnly NodeConditionType = "ReadOnly"
)

// MysqlClusterStatus defines the observed state of MysqlCluster
type MysqlClusterStatus struct {
	// ReadyNodes represents number of the nodes that are in ready state
	ReadyNodes int `json:"readyNodes,omitempty"`
	// Conditions contains the list of the cluster conditions fulfilled
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	// Nodes contains informations from orchestrator
	Nodes []NodeStatus `json:"nodes,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlCluster is the Schema for the mysqlclusters API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.readyNodes
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type == "Ready")].status",description="The cluster status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="The number of desired nodes"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MysqlCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlClusterSpec   `json:"spec,omitempty"`
	Status MysqlClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlClusterList contains a list of MysqlCluster
type MysqlClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlCluster{}, &MysqlClusterList{})
}
