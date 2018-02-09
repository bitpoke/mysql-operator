package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// MysqlClusterCRDKind is the kind of the crd that will be created.
	MysqlClusterCRDKind = "MysqlCluster"
	// MysqlClusterCRDPlural is the plural for mysqlcluster
	MysqlClusterCRDPlural = "mysqlclusters"
	groupName             = "titanium.presslabs.net"
)

var (
	// SchemeBuilder the scheme builder
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme function
	AddToScheme = SchemeBuilder.AddToScheme
	// SchemeGroupVersion ..
	SchemeGroupVersion = schema.GroupVersion{Group: groupName, Version: "v1alpha1"}
	// MysqlClusterCRDName the crd name
	MysqlClusterCRDName = MysqlClusterCRDPlural + "." + groupName
)

// Resource gets an MysqlCluster GroupResource for a specified resource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&MysqlCluster{},
		&MysqlClusterList{},
	)
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}
