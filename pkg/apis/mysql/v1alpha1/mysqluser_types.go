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

// NOTE: json tags are required. Any new fields you add must have json tags for
// the fields to be serialized.

// ClusterReference represents a cross namespace object reference
type ClusterReference struct {
	corev1.LocalObjectReference `json:",inline"`
	// Namespace the MySQL cluster namespace
	Namespace string `json:"namespace,omitempty"`
}

// MysqlUserSpec defines the desired state of MysqlUserSpec
type MysqlUserSpec struct {
	// ClusterRef represents a reference to the MySQL cluster.
	// This field should be immutable.
	ClusterRef ClusterReference `json:"clusterRef"`

	// User is the name of the user that will be created with will access the specified database.
	// This field should be immutable.
	User string `json:"user"`

	// Password is the password for the user.
	Password corev1.SecretKeySelector `json:"password"`

	// AllowedHosts is the allowed host to connect from.
	AllowedHosts []string `json:"allowedHosts"`

	// Permissions is the list of roles that user has in the specified database.
	Permissions []MysqlPermission `json:"permissions,omitempty"`

	// ResourceLimits allow settings limit per mysql user as defined here:
	// https://dev.mysql.com/doc/refman/5.7/en/user-resources.html
	// +optional
	ResourceLimits corev1.ResourceList `json:"resourceLimits,omitempty"`
}

// MysqlPermission defines a MySQL schema permission
type MysqlPermission struct {
	// Schema represents the schema to which the permission applies
	Schema string `json:"schema"`
	// Tables represents the tables inside the schema to which the permission applies
	Tables []string `json:"tables"`
	// Permissions represents the permissions granted on the schema/tables
	Permissions []string `json:"permissions"`
}

const (
	// AccountResourceMaxUserConnections it restricts the maximum number of simultaneous connections to the server
	// by each account
	AccountResourceMaxUserConnections corev1.ResourceName = "MAX_USER_CONNECTIONS"

	// AccountResourceMaxQueriesPerHour it restricts how many queries to the server
	// are permitted to each account during any given one-hour period.
	AccountResourceMaxQueriesPerHour corev1.ResourceName = "MAX_QUERIES_PER_HOUR"

	// AccountResourceMaxUpdatesPerHour it restricts how many updates to the server
	// are permitted to each account during any given one-hour period.
	AccountResourceMaxUpdatesPerHour corev1.ResourceName = "MAX_UPDATES_PER_HOUR"

	// AccountResourceMaxConnectionsPerHour it restricts how many connections to the server
	// are permitted to each account during any given one-hour period.
	AccountResourceMaxConnectionsPerHour corev1.ResourceName = "MAX_CONNECTIONS_PER_HOUR"
)

// MysqlUserConditionType defines the condition types of a MysqlUser resource
type MysqlUserConditionType string

const (
	// MySQLUserReady means the MySQL user is ready when database exists.
	MySQLUserReady MysqlUserConditionType = "Ready"
)

// MySQLUserCondition defines the condition struct for a MysqlUser resource
type MySQLUserCondition struct {
	// Type of MysqlUser condition.
	Type MysqlUserConditionType `json:"type"`
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

// MysqlUserStatus defines the observed state of MysqlUser
type MysqlUserStatus struct {
	// Conditions represents the MysqlUser resource conditions list.
	// +optional
	Conditions []MySQLUserCondition `json:"conditions,omitempty"`

	// AllowedHosts contains the list of hosts that the user is allowed to connect from.
	AllowedHosts []string `json:"allowedHosts,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlUser is the Schema for the MySQL User API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type == 'Ready')].status",description="The user status"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="UserName",type="string",JSONPath=".spec.user"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MysqlUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MysqlUserSpec   `json:"spec,omitempty"`
	Status            MysqlUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlUserList contains a list of MysqlUser
type MysqlUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlUser `json:"items,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MysqlUser{}, &MysqlUserList{})
}
