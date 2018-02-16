package mysqlcluster

import (
	"fmt"

	kapps "github.com/appscode/kutil/apps/v1"
	kcore "github.com/appscode/kutil/core/v1"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	volumeMysqlName = "mysql_data"
)

func (f *cFactory) syncMasterDeployment(rName string) (state string, err error) {
	state = statusUpToDate
	name := fmt.Sprintf("%s-%s", f.getNameForResource(MasterDeployment), rName)
	pvcName := fmt.Sprintf("%s-data", name)

	pvc, err := f.syncMasterDeploymentVolume(pvcName)
	if err != nil {
		state = statusFaild
		return
	}

	meta := metav1.ObjectMeta{
		Name: name,
		Labels: f.getLabels(map[string]string{
			"master_node": "true",
		}),
		OwnerReferences: f.getOwnerReferences(),
	}

	_, act, err := kapps.CreateOrPatchDeployment(f.client, meta,
		func(in *apps.Deployment) *apps.Deployment {
			in.Spec = f.getMasterDeploymentSpec(pvc)
			return in
		})
	if err != nil {
		state = statusFaild
		return
	}

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) syncMasterDeploymentVolume(name string) (*v1.Volume, error) {
	if f.cl.Spec.VolumeSpec.PersistenceDisabled {
		return &v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		}, nil
	}

	meta := metav1.ObjectMeta{
		Name:            name,
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
	}

	pvc, _, err := kcore.CreateOrPatchPVC(f.client, meta,
		func(in *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
			in.Spec = f.cl.Spec.VolumeSpec.PersistentVolumeClaimSpec
			return in
		})
	if err != nil {
		return nil, err
	}

	return &v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
			},
		},
	}, nil

}

func (f *cFactory) getMasterDeploymentSpec(volume *v1.Volume) apps.DeploymentSpec {
	replicas := int32(1)
	return apps.DeploymentSpec{
		Replicas: &replicas,
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{*volume},
				InitContainers: []v1.Container{
					v1.Container{
						Name:            containerInitName,
						Image:           f.cl.Spec.GetTitaniumImage(),
						ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
						Args:            []string{"files-config"},
						EnvFrom:         f.getEnvSourcesFor(containerInitName),
						VolumeMounts: []v1.VolumeMount{
							v1.VolumeMount{
								Name:      confVolumeName,
								MountPath: confVolumeMountPath,
							},
							v1.VolumeMount{
								Name:      confMapVolumeName,
								MountPath: confMapVolumeMountPath,
							},
						},
					},
				},
				Containers: []v1.Container{
					v1.Container{
						Name:            containerMysqlName,
						Image:           f.cl.Spec.GetMysqlImage(),
						ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
						EnvFrom:         f.getEnvSourcesFor(containerMysqlName),
						Ports: []v1.ContainerPort{
							v1.ContainerPort{
								Name:          MysqlPortName,
								ContainerPort: MysqlPort,
							},
						},
						Resources:      f.cl.Spec.PodSpec.Resources,
						LivenessProbe:  getLivenessProbe(),
						ReadinessProbe: getReadinessProbe(),
						VolumeMounts:   getVolumeMounts(),
					},
					v1.Container{
						Name:            containerMysqlName,
						Image:           f.cl.Spec.GetMysqlImage(),
						ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
						EnvFrom:         f.getEnvSourcesFor(containerMysqlName),
						Ports: []v1.ContainerPort{
							v1.ContainerPort{
								Name:          MysqlPortName,
								ContainerPort: MysqlPort,
							},
						},
						Resources:      f.cl.Spec.PodSpec.Resources,
						LivenessProbe:  getLivenessProbe(),
						ReadinessProbe: getReadinessProbe(),
						VolumeMounts:   getVolumeMounts(),
					},
				},
			},
		},
	}
}
