package mysqlcluster

import (
	"fmt"

	kapps "github.com/appscode/kutil/apps/v1"
	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
)

const (
	confVolumeName      = "conf"
	ConfVolumeMountPath = "/etc/mysql"
	ConfDPath           = "/etc/mysql/conf.d"

	confMapVolumeName      = "config-map"
	ConfMapVolumeMountPath = "/mnt/conf"

	DataVolumeMountPath = "/var/lib/mysql"
)

func (f *cFactory) syncStatefulSet() (state string, err error) {
	state = statusUpToDate
	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.StatefulSet),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kapps.CreateOrPatchStatefulSet(f.client, meta,
		func(in *apps.StatefulSet) *apps.StatefulSet {
			if in.Status.ReadyReplicas == in.Status.Replicas {
				f.cluster.UpdateStatusCondition(api.ClusterConditionReady,
					core.ConditionTrue, "statefulset ready", "Cluster is ready.")
			} else {
				f.cluster.UpdateStatusCondition(api.ClusterConditionReady,
					core.ConditionFalse, "statefulset not ready", "Cluster is not ready.")
			}

			f.cluster.Status.ReadyNodes = int(in.Status.ReadyReplicas)

			in.Spec.Replicas = &f.cluster.Spec.Replicas
			in.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: f.getLabels(map[string]string{}),
			}

			in.Spec.ServiceName = f.cluster.GetNameForResource(api.HeadlessSVC)
			in.Spec.Template = f.ensureTemplate(in.Spec.Template)
			if len(in.Spec.VolumeClaimTemplates) == 0 {
				in.Spec.VolumeClaimTemplates = f.getVolumeClaimTemplates()
			}

			return in
		})

	if err != nil {
		state = statusFaild
		return
	}

	state = getStatusFromKVerb(act)
	glog.V(3).Infof("SFS synced state: %s", state)
	return
}

func (f *cFactory) ensureTemplate(in core.PodTemplateSpec) core.PodTemplateSpec {
	in.ObjectMeta.Labels = f.getLabels(f.cluster.Spec.PodSpec.Labels)
	in.ObjectMeta.Annotations = f.cluster.Spec.PodSpec.Annotations
	if len(in.ObjectMeta.Annotations) == 0 {
		in.ObjectMeta.Annotations = make(map[string]string)
	}
	in.ObjectMeta.Annotations["config_hash"] = f.configHash

	in.Spec.InitContainers = f.ensureInitContainersSpec(in.Spec.InitContainers)
	in.Spec.Containers = f.ensureContainersSpec(in.Spec.Containers)

	// TODO: ensure function for volume
	if len(in.Spec.Volumes) == 0 {
		in.Spec.Volumes = f.getVolumes()
	}
	in.Spec.Affinity = &f.cluster.Spec.PodSpec.Affinity
	in.Spec.NodeSelector = f.cluster.Spec.PodSpec.NodeSelector
	in.Spec.ImagePullSecrets = f.cluster.Spec.PodSpec.ImagePullSecrets

	return in
}

const (
	containerInitName     = "init-mysql"
	containerCloneName    = "clone-mysql"
	containerTitaniumName = "titanium"
	containerMysqlName    = "mysql"
)

func (f *cFactory) ensureContainer(in core.Container, name, image string, args []string) core.Container {
	in.Name = name
	in.Image = image
	in.ImagePullPolicy = f.cluster.Spec.PodSpec.ImagePullPolicy
	in.Args = args
	in.EnvFrom = f.getEnvSourcesFor(name)
	in.VolumeMounts = f.getVolumeMountsFor(name)

	return in
}

func (f *cFactory) ensureInitContainersSpec(in []core.Container) []core.Container {
	if len(in) == 0 {
		in = make([]core.Container, 2)
	}

	// init container for configs
	in[0] = f.ensureContainer(in[0], containerInitName,
		f.cluster.Spec.GetTitaniumImage(),
		[]string{"files-config"},
	)

	// clone container
	in[1] = f.ensureContainer(in[1], containerCloneName,
		f.cluster.Spec.GetTitaniumImage(),
		[]string{"clone"},
	)

	return in
}

func (f *cFactory) ensureContainersSpec(in []core.Container) []core.Container {
	if len(in) == 0 {
		in = make([]core.Container, 2)
	}

	// MYSQL container
	mysql := f.ensureContainer(in[0], containerMysqlName,
		f.cluster.Spec.GetMysqlImage(),
		[]string{},
	)
	mysql.Ports = ensureContainerPorts(mysql.Ports, core.ContainerPort{
		Name:          MysqlPortName,
		ContainerPort: MysqlPort,
	})
	mysql.Resources = f.cluster.Spec.PodSpec.Resources
	mysql.LivenessProbe = ensureProbe(mysql.LivenessProbe, 30, 5, 10, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				"--defaults-file=/etc/mysql/client.cnf",
				"ping",
			},
		},
	})

	mysql.ReadinessProbe = ensureProbe(mysql.ReadinessProbe, 5, 5, 10, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysql",
				"--defaults-file=/etc/mysql/client.cnf",
				"-e",
				"SELECT 1",
			},
		},
	})
	in[0] = mysql

	titanium := f.ensureContainer(in[1], containerTitaniumName,
		f.cluster.Spec.GetTitaniumImage(),
		[]string{"config-and-serve"},
	)
	titanium.Ports = ensureContainerPorts(titanium.Ports, core.ContainerPort{
		Name:          TitaniumXtrabackupPortName,
		ContainerPort: TitaniumXtrabackupPort,
	})

	// TITANIUM container
	titanium.ReadinessProbe = ensureProbe(titanium.ReadinessProbe, 5, 5, 10, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path: TitaniumProbePath,
			Port: intstr.FromInt(TitaniumProbePort),
		},
	})
	in[1] = titanium

	return in
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
						Name: f.cluster.GetNameForResource(api.ConfigMap),
					},
				},
			},
		},
		core.Volume{
			Name: f.cluster.GetNameForResource(api.VolumePVC),
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: f.cluster.GetNameForResource(api.VolumePVC),
				},
			},
		},
	}
	if len(f.cluster.Spec.GetOrcTopologySecret()) != 0 {
		volumes = append(volumes, core.Volume{
			Name: "orc-topology-secret",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: f.cluster.Spec.GetOrcTopologySecret(),
				},
			},
		})
	}

	return volumes
}

func (f *cFactory) getVolumeClaimTemplates() []core.PersistentVolumeClaim {
	return []core.PersistentVolumeClaim{
		core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            f.cluster.GetNameForResource(api.VolumePVC),
				Labels:          f.getLabels(map[string]string{}),
				OwnerReferences: f.getOwnerReferences(),
			},
			Spec: f.cluster.Spec.VolumeSpec.PersistentVolumeClaimSpec,
		},
	}
}

func (f *cFactory) getEnvSourcesFor(name string) []core.EnvFromSource {
	ss := []core.EnvFromSource{
		envFromSecret(f.cluster.GetNameForResource(api.EnvSecret)),
	}
	switch name {
	case containerTitaniumName:
		//		if len(f.cluster.Spec.BackupBucketSecretName) != 0 {
		//			ss = append(ss, envFromSecret(f.cluster.Spec.BackupBucketSecretName))
		//		}
	case containerCloneName:
		if len(f.cluster.Spec.InitBucketSecretName) != 0 {
			ss = append(ss, envFromSecret(f.cluster.Spec.InitBucketSecretName))
		}
	case containerMysqlName:
		ss = append(ss, core.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: f.cluster.Spec.SecretName,
				},
			},
		})
	}
	return ss
}

func (f *cFactory) getVolumeMountsFor(name string) []core.VolumeMount {
	commonVolumeMounts := []core.VolumeMount{
		core.VolumeMount{
			Name:      confVolumeName,
			MountPath: ConfVolumeMountPath,
		},
		core.VolumeMount{
			Name:      f.cluster.GetNameForResource(api.VolumePVC),
			MountPath: DataVolumeMountPath,
		},
	}

	titaniumVolumeMounts := commonVolumeMounts
	if len(f.cluster.Spec.GetOrcTopologySecret()) != 0 {
		titaniumVolumeMounts = append(commonVolumeMounts, core.VolumeMount{
			Name:      "orc-topology-secret",
			MountPath: OrcTopologyDir,
		})
	}
	switch name {
	case containerInitName:
		return []core.VolumeMount{
			core.VolumeMount{
				Name:      confVolumeName,
				MountPath: ConfVolumeMountPath,
			},
			core.VolumeMount{
				Name:      confMapVolumeName,
				MountPath: ConfMapVolumeMountPath,
			},
		}

	case containerCloneName:
		return commonVolumeMounts

	case containerMysqlName:
		return commonVolumeMounts

	case containerTitaniumName:
		return titaniumVolumeMounts
	}
	return nil
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

func (f *cFactory) getHostForReplica(no int) string {
	return fmt.Sprintf("%s-%d.%s", f.cluster.GetNameForResource(api.StatefulSet), no,
		f.cluster.GetNameForResource(api.HeadlessSVC))
}
