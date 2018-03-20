package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"github.com/presslabs/titanium/pkg/util/options"
)

func (b *MysqlBackup) UpdateDefaults() error {
	if len(b.Status.BucketUri) == 0 {
		bucketPrefix := b.Spec.BucketUri
		if strings.HasSuffix(bucketPrefix, "/") {
			bucketPrefix = bucketPrefix[:len(bucketPrefix)-1]
		}
		t := time.Now()
		b.Status.BucketUri = bucketPrefix + fmt.Sprintf(
			"/%s-%s.xbackup.gz", b.ClusterName, t.Format("2006-01-02T15:04:05"),
		)
	}

	return nil
}

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
