package clustercontroller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func (c *Controller) instanceForOwnerReference(objectMeta *metav1.ObjectMeta) (*api.MysqlCluster, error) {

	owner := metav1.GetControllerOf(objectMeta)
	if owner == nil {
		return nil, fmt.Errorf("resource does not have a controller.")
	}

	if owner.Kind != api.MysqlClusterKind || owner.APIVersion != api.SchemeGroupVersion.String() {
		return nil, fmt.Errorf("reference is not mysql cluster resource")
	}

	cluster, err := c.clusterLister.MysqlClusters(objectMeta.Namespace).Get(owner.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting reference for cluster, err: %s", err)
	}

	return cluster, nil
}
