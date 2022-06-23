package v1alpha1

type MysqlResourceDeletionPolicy string

const (
	MysqlResourceDeletionPolicyAnnotationKey = "mysql-operator.presslabs.org/resourceDeletionPolicy"
	MysqlResourceDeletionPolicyDelete        = MysqlResourceDeletionPolicy("delete")
	MysqlResourceDeletionPolicyRetain        = MysqlResourceDeletionPolicy("retain")
)
