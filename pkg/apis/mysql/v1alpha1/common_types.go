package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

type MysqlResourceDeletionPolicy string

const (
	MysqlResourceDeletionPolicyAnnotationKey = "mysql-operator.presslabs.org/resourceDeletionPolicy"
	MysqlResourceDeletionPolicyDelete        = MysqlResourceDeletionPolicy("delete")
	MysqlResourceDeletionPolicyRetain        = MysqlResourceDeletionPolicy("retain")
)

func CheckResourceDeletionPolicyRetain(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	return annotations != nil && MysqlResourceDeletionPolicy(annotations[MysqlResourceDeletionPolicyAnnotationKey]) == MysqlResourceDeletionPolicyRetain
}
