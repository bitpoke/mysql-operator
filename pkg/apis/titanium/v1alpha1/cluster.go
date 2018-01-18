package v1alpha1

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util"
)

const (
	InnodbBufferSizePercent = 80
)

func (c *MysqlCluster) AsOwnerReference() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       MysqlClusterCRDKind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}

func (c *MysqlCluster) UpdateDefaults() error {
	return c.Spec.UpdateDefaults()
}

func (c *ClusterSpec) UpdateDefaults() error {
	if len(c.MysqlRootPassword) == 0 {
		return fmt.Errorf("mysqlRootPassword is requred!")
	}

	if len(c.MysqlReplicationUser) == 0 || len(c.MysqlReplicationPassword) == 0 {
		c.MysqlReplicationUser = util.RandomString(8)
		c.MysqlReplicationPassword = util.RandomString(12)
	}

	if len(c.MysqlUser) > 0 {
		if len(c.MysqlPassword) == 0 {
			return fmt.Errorf("mysqlPassword is required if mysqlUser is set.")
		}
		if len(c.MysqlDatabase) == 0 {
			return fmt.Errorf("mysqlDatabase is required if mysqlUser is set.")
		}
	}

	if err := c.PodSpec.UpdateDefaults(); err != nil {
		return err
	}

	// set innodb-buffer-pool-size as 80% of requested memory
	if _, ok := c.MysqlConfig["innodb-buffer-pool-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			val := (InnodbBufferSizePercent * mem.Value()) / 100 // val is 80% of requested memory
			res := resource.NewQuantity(val, resource.DecimalSI)
			if len(c.MysqlConfig) == 0 {
				c.MysqlConfig = make(MysqlConfig)
			}
			// TODO: make it human readable
			c.MysqlConfig["innodb-buffer-pool-size"] = res.String()
		}
	}

	return nil
}

const (
	MysqlImage      = "percona:5.7"
	ImagePullPolicy = apiv1.PullIfNotPresent

	TitaniumImage           = "gcr.io/pl-infra/titanium-toolbox:latest"
	TitaniumImagePullPolicy = apiv1.PullIfNotPresent

	MetricsImage           = ""
	MetricsImagePullPolicy = apiv1.PullIfNotPresent

	ResourceRequestCPU    = "200m"
	ResourceRequestMemory = "1Gi"
)

func (ps *PodSpec) UpdateDefaults() error {
	if len(ps.Image) == 0 {
		ps.Image = MysqlImage
	}

	if len(ps.ImagePullPolicy) == 0 {
		ps.ImagePullPolicy = ImagePullPolicy
	}

	if len(ps.TitaniumImage) == 0 {
		ps.TitaniumImage = TitaniumImage
	}

	if len(ps.TitaniumImagePullPolicy) == 0 {
		ps.TitaniumImagePullPolicy = TitaniumImagePullPolicy
	}

	if len(ps.MetricsImage) == 0 {
		ps.MetricsImage = MetricsImage
	}

	if len(ps.MetricsImagePullPolicy) == 0 {
		ps.MetricsImagePullPolicy = MetricsImagePullPolicy
	}

	if len(ps.Resources.Requests) == 0 {
		ps.Resources.Requests = apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse(ResourceRequestCPU),
			apiv1.ResourceMemory: resource.MustParse(ResourceRequestMemory),
		}
	}
	return nil
}
