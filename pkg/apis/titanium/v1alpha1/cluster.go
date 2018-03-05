package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/presslabs/titanium/pkg/util/options"
)

const (
	innodbBufferSizePercent = 80
)

var (
	opt *options.Options
)

func init() {
	opt = options.GetOptions()
}

// AsOwnerReference returns the MysqlCluster owner references.
func (c *MysqlCluster) AsOwnerReference() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       MysqlClusterKind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}

// UpdateDefaults sets the defaults for Spec and Status
func (c *MysqlCluster) UpdateDefaults(opt *options.Options) error {
	c.UpdateStatusCondition(ClusterConditionReady,
		apiv1.ConditionUnknown, "not initialized", "setting defaults")

	return c.Spec.UpdateDefaults(opt)
}

// UpdateDefaults updates Spec defaults
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
			val := (innodbBufferSizePercent * mem.Value()) / 100 // val is 80% of requested memory
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

// GetTitaniumImage return titanium image from options
func (c *ClusterSpec) GetTitaniumImage() string {
	return opt.TitaniumImage
}

// GetOrcUri return the orchestrator uri
func (c *ClusterSpec) GetOrcUri() string {
	return opt.OrchestratorUri
}

// GetOrcTopologySecret return the name of the secret that contains the
// credentaials for orc to connect to mysql nodes.
func (c *ClusterSpec) GetOrcTopologySecret() string {
	return opt.OrchestratorTopologySecretName
}

// GetMysqlImage returns mysql image, composed from oprions and  Spec.MysqlVersion
func (c *ClusterSpec) GetMysqlImage() string {
	return opt.MysqlImage + ":" + c.MysqlVersion
}

const (
	resourceRequestCPU    = "200m"
	resourceRequestMemory = "1Gi"

	resourceStorage = "8Gi"
)

// UpdateDefaults for PodSpec
func (ps *PodSpec) UpdateDefaults(opt *options.Options) error {
	if len(ps.ImagePullPolicy) == 0 {
		ps.ImagePullPolicy = opt.ImagePullPolicy
	}

	// TODO: check if are applied or write a test for thoses
	if len(ps.Resources.Requests) == 0 {
		ps.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceCPU:    resource.MustParse(resourceRequestCPU),
				apiv1.ResourceMemory: resource.MustParse(resourceRequestMemory),
			},
		}
	}
	return nil
}

// UpdateDefaults for VolumeSpec
func (vs *VolumeSpec) UpdateDefaults() error {
	if len(vs.AccessModes) == 0 {
		vs.AccessModes = []apiv1.PersistentVolumeAccessMode{
			apiv1.ReadWriteOnce,
		}
	}

	if len(vs.Resources.Requests) == 0 {
		vs.Resources = apiv1.ResourceRequirements{
			Requests: apiv1.ResourceList{
				apiv1.ResourceStorage: resource.MustParse(resourceStorage),
			},
		}
	}

	return nil
}
