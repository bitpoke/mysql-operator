package mysqlcluster

import (
	"fmt"

	"k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfVolumeName      = "conf"
	ConfVolumeMountPath = "/etc/mysql"

	ConfMapVolumeName      = "config-map"
	ConfMapVolumeMountPath = "/mnt/config-map"

	InitSecretVolumeName      = "init-secrets"
	InitSecretVolumeMountPath = "/var/run/secrets/buckets"

	DataVolumeName      = "data"
	DataVolumeMountPath = "/var/lib/mysql"
)

func (c *cluster) createStatefulSet() v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(StatefulSet),
			Labels:          c.getLabels(map[string]string{}),
			OwnerReferences: c.getOwnerReferences(),
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &c.cl.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getLabels(map[string]string{}),
			},
			ServiceName:          c.getNameForResource(HeadlessSVC),
			Template:             c.getPodTempalteSpec(),
			VolumeClaimTemplates: c.getVolumeClaimTemplates(),
		},
	}
}

func (c *cluster) getPodTempalteSpec() apiv1.PodTemplateSpec {
	return apiv1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.getNameForResource(SSPod),
			Labels:          c.getLabels(c.cl.Spec.PodSpec.Labels),
			OwnerReferences: c.getOwnerReferences(),
			Annotations:     c.cl.Spec.PodSpec.Annotations,
		},
		Spec: apiv1.PodSpec{
			InitContainers: c.getInitContainersSpec(),
			Containers:     c.getContainersSpec(),
			Volumes:        c.getVolumes(),

			Affinity:         &c.cl.Spec.PodSpec.Affinity,
			NodeSelector:     c.cl.Spec.PodSpec.NodeSelector,
			ImagePullSecrets: c.cl.Spec.PodSpec.ImagePullSecrets,
		},
	}
}

func (c *cluster) getInitContainersSpec() []apiv1.Container {
	initCoVM := []apiv1.VolumeMount{
		apiv1.VolumeMount{
			Name:      ConfVolumeName,
			MountPath: ConfVolumeMountPath,
		},
		apiv1.VolumeMount{
			Name:      ConfMapVolumeName,
			MountPath: ConfMapVolumeMountPath,
		},
	}
	if c.existsSecret(c.cl.Spec.InitBucketSecretName) {
		initCoVM = append(initCoVM, apiv1.VolumeMount{
			Name:      InitSecretVolumeName,
			MountPath: InitSecretVolumeMountPath,
		})
	}
	return []apiv1.Container{
		apiv1.Container{
			Name:            "init-mysql",
			Image:           c.cl.Spec.PodSpec.TitaniumImage,
			ImagePullPolicy: c.cl.Spec.PodSpec.TitaniumImagePullPolicy,
			Args:            []string{"files-config"},
			EnvFrom: []apiv1.EnvFromSource{
				apiv1.EnvFromSource{
					SecretRef: &apiv1.SecretEnvSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: c.getNameForResource(EnvSecret),
						},
					},
				},
			},
			VolumeMounts: initCoVM,
		},
		apiv1.Container{
			Name:            "clone-mysql",
			Image:           c.cl.Spec.PodSpec.TitaniumImage,
			ImagePullPolicy: c.cl.Spec.PodSpec.TitaniumImagePullPolicy,
			Args:            []string{"clone"},
			EnvFrom: []apiv1.EnvFromSource{
				apiv1.EnvFromSource{
					SecretRef: &apiv1.SecretEnvSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: c.getNameForResource(EnvSecret),
						},
					},
				},
			},
			VolumeMounts: getVolumeMounts(),
		},
	}
}

func (c *cluster) getContainersSpec() []apiv1.Container {
	return []apiv1.Container{
		apiv1.Container{
			Name:            "mysql",
			Image:           c.cl.Spec.PodSpec.Image,
			ImagePullPolicy: c.cl.Spec.PodSpec.ImagePullPolicy,
			EnvFrom: []apiv1.EnvFromSource{
				apiv1.EnvFromSource{
					SecretRef: &apiv1.SecretEnvSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: c.getNameForResource(EnvSecret),
						},
					},
				},
			},
			Ports: []apiv1.ContainerPort{
				apiv1.ContainerPort{
					Name:          MysqlPortName,
					HostPort:      MysqlPort,
					ContainerPort: MysqlPort,
				},
			},
			Resources:      c.cl.Spec.PodSpec.Resources,
			LivenessProbe:  getLivenessProbe(),
			ReadinessProbe: getReadinessProbe(),
			VolumeMounts:   getVolumeMounts(),
		},
		apiv1.Container{
			Name:  "titanium",
			Image: c.cl.Spec.PodSpec.TitaniumImage,
			Args:  []string{"config-and-serve"},
			EnvFrom: []apiv1.EnvFromSource{
				apiv1.EnvFromSource{
					SecretRef: &apiv1.SecretEnvSource{
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: c.getNameForResource(EnvSecret),
						},
					},
				},
			},
			Ports: []apiv1.ContainerPort{
				apiv1.ContainerPort{
					Name:          TitaniumXtrabackupPortName,
					HostPort:      TitaniumXtrabackupPort,
					ContainerPort: TitaniumXtrabackupPort,
				},
			},
			VolumeMounts: getVolumeMounts(),
		},
	}
}

func getLivenessProbe() *apiv1.Probe {
	return &apiv1.Probe{
		InitialDelaySeconds: 30,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		Handler: apiv1.Handler{
			Exec: &apiv1.ExecAction{
				Command: []string{
					"mysqladmin",
					"--defaults-file=/etc/mysql/client.cnf",
					"ping",
				},
			},
		},
	}
}

func getReadinessProbe() *apiv1.Probe {
	return &apiv1.Probe{
		InitialDelaySeconds: 5,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		Handler: apiv1.Handler{
			Exec: &apiv1.ExecAction{
				Command: []string{
					"mysql",
					"--defaults-file=/etc/mysql/client.cnf",
					"-e",
					"SELECT 1",
				},
			},
		},
	}
}

func (c *cluster) getVolumes() []apiv1.Volume {
	volumes := []apiv1.Volume{
		apiv1.Volume{
			Name: ConfVolumeName,
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
		apiv1.Volume{
			Name: ConfMapVolumeName,
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: c.getNameForResource(ConfigMap),
					},
				},
			},
		},

		c.getDataVolume(),
	}

	if c.existsSecret(c.cl.Spec.InitBucketSecretName) {
		volumes = append(volumes, apiv1.Volume{
			Name: InitSecretVolumeName,
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: c.cl.Spec.InitBucketSecretName,
				},
			},
		})
	}
	return volumes
}

func (c *cluster) getDataVolume() apiv1.Volume {
	vs := apiv1.VolumeSource{
		EmptyDir: &apiv1.EmptyDirVolumeSource{},
	}

	if !c.cl.Spec.VolumeSpec.PersistenceDisabled {
		vs = apiv1.VolumeSource{
			PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
				ClaimName: c.getNameForResource(VolumePVC),
			},
		}
	}

	return apiv1.Volume{
		Name:         DataVolumeName,
		VolumeSource: vs,
	}
}

func getVolumeMounts(extra ...apiv1.VolumeMount) []apiv1.VolumeMount {
	common := []apiv1.VolumeMount{
		apiv1.VolumeMount{
			Name:      ConfVolumeName,
			MountPath: ConfVolumeMountPath,
		},
		apiv1.VolumeMount{
			Name:      DataVolumeName,
			MountPath: DataVolumeMountPath,
		},
	}

	for _, vm := range extra {
		common = append(common, vm)
	}

	return common
}

func (c *cluster) getVolumeClaimTemplates() []apiv1.PersistentVolumeClaim {
	if c.cl.Spec.VolumeSpec.PersistenceDisabled {
		fmt.Println("Persistence is disabled.")
		return nil
	}

	return []apiv1.PersistentVolumeClaim{
		apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            c.getNameForResource(VolumePVC),
				Labels:          c.getLabels(map[string]string{}),
				OwnerReferences: c.getOwnerReferences(),
			},
			Spec: c.cl.Spec.VolumeSpec.PersistentVolumeClaimSpec,
		},
	}
}
