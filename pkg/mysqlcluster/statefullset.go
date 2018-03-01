package mysqlcluster

import (
	"fmt"

	kapps "github.com/appscode/kutil/apps/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
)

const (
	confVolumeName      = "conf"
	ConfVolumeMountPath = "/etc/mysql"

	confMapVolumeName      = "config-map"
	ConfMapVolumeMountPath = "/mnt/conf"

	dataVolumeName      = "data"
	DataVolumeMountPath = "/var/lib/mysql"
)

func (f *cFactory) syncStatefulSet() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.getNameForResource(StatefulSet),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kapps.CreateOrPatchStatefulSet(f.client, meta,
		func(in *apps.StatefulSet) *apps.StatefulSet {
			if in.Status.ReadyReplicas == in.Status.Replicas {
				f.cl.UpdateStatusCondition(api.ClusterConditionReady,
					core.ConditionTrue, "statefulset ready", "Cluster is ready.")
			} else {
				f.cl.UpdateStatusCondition(api.ClusterConditionReady,
					core.ConditionFalse, "statefulset not ready", "Cluster is not ready.")
			}

			in.Spec = apps.StatefulSetSpec{
				Replicas: &f.cl.Spec.ReadReplicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: f.getLabels(map[string]string{}),
				},
				ServiceName:          f.getNameForResource(HeadlessSVC),
				Template:             f.getPodTempalteSpec(),
				VolumeClaimTemplates: f.getVolumeClaimTemplates(),
			}
			return in
		})

	if err != nil {
		state = statusFaild
		return
	}

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) getPodTempalteSpec() core.PodTemplateSpec {
	return core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      f.getLabels(f.cl.Spec.PodSpec.Labels),
			Annotations: f.cl.Spec.PodSpec.Annotations,
		},
		Spec: core.PodSpec{
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

func (f *cFactory) getInitContainersSpec() []core.Container {
	return []core.Container{
		core.Container{
			Name:            containerInitName,
			Image:           f.cl.Spec.GetTitaniumImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			Args:            []string{"files-config"},
			EnvFrom:         f.getEnvSourcesFor(containerInitName),
			VolumeMounts: []core.VolumeMount{
				core.VolumeMount{
					Name:      confVolumeName,
					MountPath: ConfVolumeMountPath,
				},
				core.VolumeMount{
					Name:      confMapVolumeName,
					MountPath: ConfMapVolumeMountPath,
				},
			},
		},
		core.Container{
			Name:            containerCloneName,
			Image:           f.cl.Spec.GetTitaniumImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			Args:            []string{"clone"},
			EnvFrom:         f.getEnvSourcesFor(containerCloneName),
			VolumeMounts:    getVolumeMounts(),
		},
	}
}

func (f *cFactory) getContainersSpec() []core.Container {
	return []core.Container{
		core.Container{
			Name:            containerMysqlName,
			Image:           f.cl.Spec.GetMysqlImage(),
			ImagePullPolicy: f.cl.Spec.PodSpec.ImagePullPolicy,
			EnvFrom:         f.getEnvSourcesFor(containerMysqlName),
			Ports: []core.ContainerPort{
				core.ContainerPort{
					Name:          MysqlPortName,
					ContainerPort: MysqlPort,
				},
			},
			Resources:      f.cl.Spec.PodSpec.Resources,
			LivenessProbe:  getLivenessProbe(),
			ReadinessProbe: getReadinessProbe(),
			VolumeMounts:   getVolumeMounts(),
		},
		core.Container{
			Name:    containerTitaniumName,
			Image:   f.cl.Spec.GetTitaniumImage(),
			Args:    []string{"config-and-serve"},
			EnvFrom: f.getEnvSourcesFor(containerTitaniumName),
			Ports: []core.ContainerPort{
				core.ContainerPort{
					Name:          TitaniumXtrabackupPortName,
					ContainerPort: TitaniumXtrabackupPort,
				},
			},
			VolumeMounts: getVolumeMounts(),
		},
	}
}

func (f *cFactory) getVolumes() []core.Volume {
	volumes := []core.Volume{
		// mysql config volume mount: /etc/mysql
		core.Volume{
			Name: confVolumeName,
			VolumeSource: core.VolumeSource{
				EmptyDir: &core.EmptyDirVolumeSource{},
			},
		},
		// config volume that contains config maps mount: /mnt/
		core.Volume{
			Name: confMapVolumeName,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.getNameForResource(ConfigMap),
					},
				},
			},
		},
	}

	// data volume mount: /var/lib/mysql
	vs := core.VolumeSource{
		EmptyDir: &core.EmptyDirVolumeSource{},
	}
	if !f.cl.Spec.VolumeSpec.PersistenceDisabled {
		vs = core.VolumeSource{
			PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
				ClaimName: f.getNameForResource(VolumePVC),
			},
		}
	}

	return append(volumes, core.Volume{
		Name:         dataVolumeName,
		VolumeSource: vs,
	})
}

func getVolumeMounts(extra ...core.VolumeMount) []core.VolumeMount {
	common := []core.VolumeMount{
		core.VolumeMount{
			Name:      confVolumeName,
			MountPath: ConfVolumeMountPath,
		},
		core.VolumeMount{
			Name:      dataVolumeName,
			MountPath: DataVolumeMountPath,
		},
	}

	for _, vm := range extra {
		common = append(common, vm)
	}

	return common
}

func (f *cFactory) getVolumeClaimTemplates() []core.PersistentVolumeClaim {
	if f.cl.Spec.VolumeSpec.PersistenceDisabled {
		fmt.Println("Persistence is disabled.")
		return nil
	}

	return []core.PersistentVolumeClaim{
		core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            f.getNameForResource(VolumePVC),
				Labels:          f.getLabels(map[string]string{}),
				OwnerReferences: f.getOwnerReferences(),
			},
			Spec: f.cl.Spec.VolumeSpec.PersistentVolumeClaimSpec,
		},
	}
}

func (f *cFactory) getEnvSourcesFor(name string) []core.EnvFromSource {
	ss := []core.EnvFromSource{
		envFromSecret(f.getNameForResource(EnvSecret)),
	}
	switch name {
	case containerTitaniumName:
		if len(f.cl.Spec.InitBucketSecretName) != 0 {
			ss = append(ss, envFromSecret(f.cl.Spec.BackupBucketSecretName))
		}
	case containerCloneName:
		if len(f.cl.Spec.BackupBucketSecretName) != 0 {
			ss = append(ss, envFromSecret(f.cl.Spec.InitBucketSecretName))
		}
	case containerMysqlName:
		ss = append(ss, core.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: f.cl.Spec.SecretName,
				},
			},
		})
	}
	return ss
}

func envFromSecret(name string) core.EnvFromSource {
	return core.EnvFromSource{
		SecretRef: &core.SecretEnvSource{
			LocalObjectReference: core.LocalObjectReference{
				Name: name,
			},
		},
	}
}
