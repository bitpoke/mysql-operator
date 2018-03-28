package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"github.com/presslabs/mysql-operator/pkg/util/options"
)

// AsOwnerReference returns the MysqlCluster owner references.
func (c *MysqlBackup) AsOwnerReference() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       MysqlBackupKind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}

func (c *MysqlBackup) GetTitaniumImage() string {
	return opt.TitaniumImage
}
