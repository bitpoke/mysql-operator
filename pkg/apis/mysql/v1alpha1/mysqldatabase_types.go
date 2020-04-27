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

// MySQLDatabaseConditionType defines the condition types of a MySQLDatabase resource
type MySQLDatabaseConditionType string

const (
	// MySQLDatabaseReady means the MySQL database is ready when database exists.
	MySQLDatabaseReady MySQLDatabaseConditionType = "Ready"
)

// MySQLDatabaseCondition defines the condition struct for a MySQLDatabase resource
type MySQLDatabaseCondition struct {
	// Type of MySQLDatabase condition.
	Type MySQLDatabaseConditionType `json:"type"`
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

// MySQLDatabaseSpec defines the desired state of MySQLDatabaseSpec
type MySQLDatabaseSpec struct {
	// ClusterRef represents a reference to the MySQL cluster.
	// This field should be immutable.
	ClusterRef ClusterReference `json:"clusterRef"`
	// Database represents the database name which will be created.
	// This field should be immutable.
	Database string `json:"database"`
}

// MySQLDatabaseStatus defines the observed state of MySQLDatabase
type MySQLDatabaseStatus struct {
	// Conditions represents the MySQLDatabase  resource conditions list.
	Conditions []MySQLDatabaseCondition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLDatabase is the Schema for the MySQL database API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type == "Ready")].status",description="The database status"
// +kubebuilder:printcolumn:name="Cluster",type="date",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Database",type="date",JSONPath=".spec.database"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MySQLDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MySQLDatabaseSpec   `json:"spec,omitempty"`
	Status            MySQLDatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MySQLDatabaseList contains a list of MySQLDatabase
type MySQLDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MySQLDatabase{}, &MySQLDatabaseList{})
}
