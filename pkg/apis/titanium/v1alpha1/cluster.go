package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util/options"
)

const (
	InnodbBufferSizePercent = 80
)

var (
	opt *options.Options
)

func init() {
	opt = options.GetOptions()
}

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

func (c *MysqlCluster) UpdateDefaults(opt *options.Options) error {
	return c.Spec.UpdateDefaults(opt)
}

func (c *ClusterSpec) UpdateDefaults(opt *options.Options) error {
	if len(c.MysqlVersion) == 0 {
		c.MysqlVersion = opt.MysqlImageTag
	}

	if err := c.PodSpec.UpdateDefaults(opt); err != nil {
		return err
	}

	// set innodb-buffer-pool-size as 80% of requested memory
	if _, ok := c.MysqlConf["innodb-buffer-pool-size"]; !ok {
		if mem := c.PodSpec.Resources.Requests.Memory(); mem != nil {
			val := (InnodbBufferSizePercent * mem.Value()) / 100 // val is 80% of requested memory
			res := resource.NewQuantity(val, resource.DecimalSI)
			if len(c.MysqlConf) == 0 {
				c.MysqlConf = make(MysqlConf)
			}
			// TODO: make it human readable
			c.MysqlConf["innodb-buffer-pool-size"] = res.String()
		}
	}

	return c.VolumeSpec.UpdateDefaults()
}

func (c *ClusterSpec) GetReplicas() *int32 {
	rep := c.ReadReplicas + 1
	return &rep
}

func (c *ClusterSpec) GetTitaniumImage() string {
	return opt.TitaniumImage
}

func (c *ClusterSpec) GetMysqlImage() string {
	return opt.MysqlImage + ":" + c.MysqlVersion
}

const (
	ResourceRequestCPU    = "200m"
	ResourceRequestMemory = "1Gi"

	ResourceStorage = "8Gi"
)

func (ps *PodSpec) UpdateDefaults(opt *options.Options) error {
	if len(ps.ImagePullPolicy) == 0 {
		ps.ImagePullPolicy = opt.ImagePullPolicy
	}

	// TODO: check if are applied or write a test for thoses
	if len(ps.Resources.Requests) == 0 {
		ps.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceCPU:    resource.MustParse(ResourceRequestCPU),
				apiv1.ResourceMemory: resource.MustParse(ResourceRequestMemory),
			},
		}
	}
	return nil
}

func (vs *VolumeSpec) UpdateDefaults() error {
	if len(vs.AccessModes) == 0 {
		vs.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
	}

	if len(vs.Resources.Requests) == 0 {
		vs.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceStorage: resource.MustParse(ResourceStorage),
			},
		}
	}

	return nil
}
