package mysqlcluster

import (
	"fmt"

	"k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	confVolumeName      = "conf"
	confVolumeMountPath = "/etc/mysql"

	confMapVolumeName      = "config-map"
	confMapVolumeMountPath = "/mnt/config-map"

	dataVolumeName      = "data"
	dataVolumeMountPath = "/var/lib/mysql"
)

func (f *cFactory) createStatefulSet() v1.StatefulSet {
	return v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            f.getNameForResource(StatefulSet),
			Labels:          f.getLabels(map[string]string{}),
			OwnerReferences: f.getOwnerReferences(),
		},
		Spec: v1.StatefulSetSpec{
			Replicas: f.cl.Spec.GetReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: f.getLabels(map[string]string{}),
			},
			ServiceName:          f.getNameForResource(HeadlessSVC),
			Template:             f.getPodTempalteSpec(),
			VolumeClaimTemplates: f.getVolumeClaimTemplates(),
		},
	}
}

func (f *cFactory) getPodTempalteSpec() apiv1.PodTemplateSpec {
	return apiv1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			//Name:        f.getNameForResource(SSPod),
			Labels:      f.getLabels(f.cl.Spec.PodSpec.Labels),
			Annotations: f.cl.Spec.PodSpec.Annotations,
		},
		Spec: apiv1.PodSpec{
			InitContainers: f.getInitContainersSpec(),
			Containers:     f.getContainersSpec(),
			Volumes:        f.getVolumes(),

			Affinity:         &f.cl.Spec.PodSpec.Affinity,
			NodeSelector:     f.cl.Spec.PodSpec.NodeSelector,
			ImagePullSecrets: f.cl.Spec.PodSpec.ImagePullSecrets,
		},
	}
}

const (
	containerInitName     = "init-mysql"
	containerCloneName    = "clone-mysql"
	containerTitaniumName = "titanium"
	containerMysqlName    = "mysql"
)

func (f *cFactory) getInitContainersSpec() []apiv1.Container {
	return []apiv1.Container{
		apiv1.Container{
			Name:            containerInitName,
			Image:           f.cl.Spec.GetTitaniumImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			Args:            []string{"files-config"},
			EnvFrom:         f.getEnvSourcesFor(containerInitName),
			VolumeMounts: []apiv1.VolumeMount{
				apiv1.VolumeMount{
					Name:      confVolumeName,
					MountPath: confVolumeMountPath,
				},
				apiv1.VolumeMount{
					Name:      confMapVolumeName,
					MountPath: confMapVolumeMountPath,
				},
			},
		},
		apiv1.Container{
			Name:            containerCloneName,
			Image:           f.cl.Spec.GetTitaniumImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			Args:            []string{"clone"},
			EnvFrom:         f.getEnvSourcesFor(containerCloneName),
			VolumeMounts:    getVolumeMounts(),
		},
	}
}

func (f *cFactory) getContainersSpec() []apiv1.Container {
	return []apiv1.Container{
		apiv1.Container{
			Name:            containerMysqlName,
			Image:           f.cl.Spec.GetMysqlImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			EnvFrom:         f.getEnvSourcesFor(containerMysqlName),
			Ports: []apiv1.ContainerPort{
				apiv1.ContainerPort{
					Name:          MysqlPortName,
					ContainerPort: MysqlPort,
				},
			},
			Resources:      f.cl.Spec.PodSpec.Resources,
			LivenessProbe:  getLivenessProbe(),
			ReadinessProbe: getReadinessProbe(),
			VolumeMounts:   getVolumeMounts(),
		},
		apiv1.Container{
			Name:    containerTitaniumName,
			Image:   f.cl.Spec.GetTitaniumImage(),
			Args:    []string{"config-and-serve"},
			EnvFrom: f.getEnvSourcesFor(containerTitaniumName),
			Ports: []apiv1.ContainerPort{
				apiv1.ContainerPort{
					Name:          TitaniumXtrabackupPortName,
					ContainerPort: TitaniumXtrabackupPort,
				},
			},
			VolumeMounts: getVolumeMounts(),
		},
	}
}

func (f *cFactory) getVolumes() []apiv1.Volume {
	return []apiv1.Volume{
		apiv1.Volume{
			Name: confVolumeName,
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
		apiv1.Volume{
			Name: confMapVolumeName,
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: f.getNameForResource(ConfigMap),
					},
				},
			},
		},

		f.getDataVolume(),
	}
}

func (f *cFactory) getDataVolume() apiv1.Volume {
	vs := apiv1.VolumeSource{
		EmptyDir: &apiv1.EmptyDirVolumeSource{},
	}

	if !f.cl.Spec.VolumeSpec.PersistenceDisabled {
		vs = apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: f.getNameForResource(VolumePVC),
			},
		}
	}

	return apiv1.Volume{
		Name:         dataVolumeName,
		VolumeSource: vs,
	}
}

func getVolumeMounts(extra ...apiv1.VolumeMount) []apiv1.VolumeMount {
	common := []apiv1.VolumeMount{
		apiv1.VolumeMount{
			Name:      confVolumeName,
			MountPath: confVolumeMountPath,
		},
		apiv1.VolumeMount{
			Name:      dataVolumeName,
			MountPath: dataVolumeMountPath,
		},
	}

	for _, vm := range extra {
		common = append(common, vm)
	}

	return common
}

func (f *cFactory) getVolumeClaimTemplates() []apiv1.PersistentVolumeClaim {
	if f.cl.Spec.VolumeSpec.PersistenceDisabled {
		fmt.Println("Persistence is disabled.")
		return nil
	}

	return []apiv1.PersistentVolumeClaim{
		apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            f.getNameForResource(VolumePVC),
				Labels:          f.getLabels(map[string]string{}),
				OwnerReferences: f.getOwnerReferences(),
			},
			Spec: f.cl.Spec.VolumeSpec.PersistentVolumeClaimSpec,
		},
	}
}

func envFromSecret(name string) apiv1.EnvFromSource {
	return apiv1.EnvFromSource{
		SecretRef: &apiv1.SecretEnvSource{
			LocalObjectReference: apiv1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func (f *cFactory) getEnvSourcesFor(name string) []apiv1.EnvFromSource {
	ss := []apiv1.EnvFromSource{
		apiv1.EnvFromSource{
			SecretRef: &apiv1.SecretEnvSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: f.getNameForResource(EnvSecret),
				},
			},
		},
	}
	switch name {
	case containerTitaniumName:
		ss = append(ss, apiv1.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &apiv1.SecretEnvSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: f.cl.Spec.SecretName,
				},
			},
		})
		if len(f.cl.Spec.InitBucketSecretName) != 0 {
			ss = append(ss, apiv1.EnvFromSource{
				SecretRef: &apiv1.SecretEnvSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: f.cl.Spec.BackupBucketSecretName,
					},
				},
			})
		}
	case containerInitName:
		ss = append(ss, apiv1.EnvFromSource{
			SecretRef: &apiv1.SecretEnvSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: f.getNameForResource(UtilitySecret),
				},
			},
		})
	case containerCloneName:
		ss = append(ss, apiv1.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &apiv1.SecretEnvSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: f.cl.Spec.SecretName,
				},
			},
		})
		if len(f.cl.Spec.BackupBucketSecretName) != 0 {
			ss = append(ss, apiv1.EnvFromSource{
				SecretRef: &apiv1.SecretEnvSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: f.cl.Spec.InitBucketSecretName,
					},
				},
			})
		}
	case containerMysqlName:
		ss = append(ss, apiv1.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &apiv1.SecretEnvSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: f.cl.Spec.SecretName,
				},
			},
		})
	}
	return ss
}
