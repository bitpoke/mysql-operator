package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

// MysqlResourceDeletionPolicy delete policy defined
type MysqlResourceDeletionPolicy string

const (
	// MysqlResourceDeletionPolicyAnnotationKey delete policy key defined
	MysqlResourceDeletionPolicyAnnotationKey = "mysql-operator.presslabs.org/resourceDeletionPolicy"
	// MysqlResourceDeletionPolicyDelete delete policy delete
	MysqlResourceDeletionPolicyDelete = MysqlResourceDeletionPolicy("delete")
	// MysqlResourceDeletionPolicyRetain delete policy retain
	MysqlResourceDeletionPolicyRetain = MysqlResourceDeletionPolicy("retain")
)

// DeletionPolicyRetain parse obj is need to delete
func DeletionPolicyRetain(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	return annotations != nil && MysqlResourceDeletionPolicy(annotations[MysqlResourceDeletionPolicyAnnotationKey]) == MysqlResourceDeletionPolicyRetain
}
