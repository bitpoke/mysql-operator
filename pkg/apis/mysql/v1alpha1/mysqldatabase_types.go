/*
Copyright 2020 Pressinfra SRL

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for
// the fields to be serialized.

// MysqlDatabaseConditionType defines the condition types of a MysqlDatabase resource
type MysqlDatabaseConditionType string

const (
	// MysqlDatabaseReady means the MySQL database is ready when database exists.
	MysqlDatabaseReady MysqlDatabaseConditionType = "Ready"
)

// MysqlDatabaseCondition defines the condition struct for a MysqlDatabase resource
type MysqlDatabaseCondition struct {
	// Type of MysqlDatabase condition.
	Type MysqlDatabaseConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

// MysqlDatabaseSpec defines the desired state of MysqlDatabaseSpec
type MysqlDatabaseSpec struct {
	// ClusterRef represents a reference to the MySQL cluster.
	// This field should be immutable.
	ClusterRef ClusterReference `json:"clusterRef"`
	// Database represents the database name which will be created.
	// This field should be immutable.
	Database string `json:"database"`
}

// MysqlDatabaseStatus defines the observed state of MysqlDatabase
type MysqlDatabaseStatus struct {
	// Conditions represents the MysqlDatabase  resource conditions list.
	Conditions []MysqlDatabaseCondition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlDatabase is the Schema for the MySQL database API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type == 'Ready')].status",description="The database status"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Database",type="string",JSONPath=".spec.database"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MysqlDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MysqlDatabaseSpec   `json:"spec,omitempty"`
	Status            MysqlDatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlDatabaseList contains a list of MysqlDatabase
type MysqlDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlDatabase{}, &MysqlDatabaseList{})
}
