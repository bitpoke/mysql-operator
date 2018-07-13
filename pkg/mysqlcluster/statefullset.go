/*
Copyright 2018 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysqlcluster

import (
	"fmt"

	kapps "github.com/appscode/kutil/apps/v1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const (
	confVolumeName      = "conf"
	ConfVolumeMountPath = "/etc/mysql"
	ConfDPath           = "/etc/mysql/conf.d"

	confMapVolumeName      = "config-map"
	ConfMapVolumeMountPath = "/mnt/conf"

	dataVolumeName      = "data"
	DataVolumeMountPath = "/var/lib/mysql"

	orcSecretVolumeName = "orc-topology-secret"
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
			in.Spec.VolumeClaimTemplates = f.ensureVolumeClaimTemplates(in.Spec.VolumeClaimTemplates)

			return in
		})

	if err != nil {
		state = statusFailed
		return
	}

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) ensureTemplate(in core.PodTemplateSpec) core.PodTemplateSpec {
	in.ObjectMeta.Labels = f.getLabels(f.cluster.Spec.PodSpec.Labels)
	in.ObjectMeta.Annotations = f.cluster.Spec.PodSpec.Annotations
	if len(in.ObjectMeta.Annotations) == 0 {
		in.ObjectMeta.Annotations = make(map[string]string)
	}
	in.ObjectMeta.Annotations["config_hash"] = f.configHash
	in.ObjectMeta.Annotations["secret_hash"] = f.secretHash
	in.ObjectMeta.Annotations["prometheus.io/scrape"] = "true"
	in.ObjectMeta.Annotations["prometheus.io/port"] = fmt.Sprintf("%d", ExporterPort)

	in.Spec.InitContainers = f.ensureInitContainersSpec(in.Spec.InitContainers)
	in.Spec.Containers = f.ensureContainersSpec(in.Spec.Containers)

	in.Spec.Volumes = f.ensureVolumes(in.Spec.Volumes)

	in.Spec.Affinity = &f.cluster.Spec.PodSpec.Affinity
	in.Spec.NodeSelector = f.cluster.Spec.PodSpec.NodeSelector
	in.Spec.ImagePullSecrets = f.cluster.Spec.PodSpec.ImagePullSecrets

	return in
}

const (
	containerInitName      = "init-mysql"
	containerCloneName     = "clone-mysql"
	containerHelperName    = "helper"
	containerMysqlName     = "mysql"
	containerExporterName  = "metrics-exporter"
	containerHeartBeatName = "pt-heartbeat"
	containerKillerName    = "pt-kill"
)

func (f *cFactory) ensureContainer(in core.Container, name, image string, args []string) core.Container {
	in.Name = name
	in.Image = image
	in.ImagePullPolicy = f.cluster.Spec.PodSpec.ImagePullPolicy
	in.Args = args
	in.EnvFrom = f.getEnvSourcesFor(name)
	in.Env = f.getEnvFor(name)
	in.VolumeMounts = f.getVolumeMountsFor(name)

	return in
}

func (f *cFactory) getEnvFor(name string) (env []core.EnvVar) {
	boolTrue := true

	env = append(env, core.EnvVar{
		Name: "MY_NAMESPACE",
		ValueFrom: &core.EnvVarSource{
			FieldRef: &core.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.namespace",
			},
		},
	})
	env = append(env, core.EnvVar{
		Name: "MY_POD_NAME",
		ValueFrom: &core.EnvVarSource{
			FieldRef: &core.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.name",
			},
		},
	})
	env = append(env, core.EnvVar{
		Name: "MY_POD_IP",
		ValueFrom: &core.EnvVarSource{
			FieldRef: &core.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "status.podIP",
			},
		},
	})
	env = append(env, core.EnvVar{
		Name:  "MY_SERVICE_NAME",
		Value: f.cluster.GetNameForResource(api.HeadlessSVC),
	})
	env = append(env, core.EnvVar{
		Name:  "MY_CLUSTER_NAME",
		Value: f.cluster.Name,
	})
	env = append(env, core.EnvVar{
		Name:  "MY_FQDN",
		Value: "$(MY_POD_NAME).$(MY_SERVICE_NAME).$(MY_NAMESPACE)",
	})
	env = append(env, core.EnvVar{
		Name:  "ORCHESTRATOR_URI",
		Value: f.cluster.Spec.GetOrcUri(),
	})

	if len(f.cluster.Spec.InitBucketUri) > 0 && name == containerCloneName {
		env = append(env, core.EnvVar{
			Name:  "INIT_BUCKET_URI",
			Value: f.cluster.Spec.InitBucketUri,
		})
	}

	switch name {
	case containerExporterName:
		env = append(env, core.EnvVar{
			Name: "USER",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key: "METRICS_EXPORTER_USER",
				},
			},
		})
		env = append(env, core.EnvVar{
			Name: "PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key: "METRICS_EXPORTER_PASSWORD",
				},
			},
		})
		env = append(env, core.EnvVar{
			Name:  "DATA_SOURCE_NAME",
			Value: fmt.Sprintf("$(USER):$(PASSWORD)@(127.0.0.1:%d)/", MysqlPort),
		})
	case containerMysqlName:
		env = append(env, core.EnvVar{
			Name: "MYSQL_ROOT_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key: "ROOT_PASSWORD",
				},
			},
		})
		env = append(env, core.EnvVar{
			Name: "MYSQL_USER",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key:      "USER",
					Optional: &boolTrue,
				},
			},
		})
		env = append(env, core.EnvVar{
			Name: "MYSQL_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key:      "PASSWORD",
					Optional: &boolTrue,
				},
			},
		})
		env = append(env, core.EnvVar{
			Name: "MYSQL_DATABASE",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key:      "DATABASE",
					Optional: &boolTrue,
				},
			},
		})
	case containerCloneName:
		env = append(env, core.EnvVar{
			Name: "MYSQL_BACKUP_USER",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key:      "BACKUP_USER",
					Optional: &boolTrue,
				},
			},
		})
		env = append(env, core.EnvVar{
			Name: "MYSQL_BACKUP_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.cluster.Spec.SecretName,
					},
					Key:      "BACKUP_PASSWORD",
					Optional: &boolTrue,
				},
			},
		})
	}

	return
}

func (f *cFactory) ensureInitContainersSpec(in []core.Container) []core.Container {
	if len(in) == 0 {
		in = make([]core.Container, 2)
	}

	// init container for configs
	in[0] = f.ensureContainer(in[0], containerInitName,
		f.cluster.Spec.GetHelperImage(),
		[]string{"files-config"},
	)

	// clone container
	in[1] = f.ensureContainer(in[1], containerCloneName,
		f.cluster.Spec.GetHelperImage(),
		[]string{"clone"},
	)

	return in
}

func (f *cFactory) ensureContainersSpec(in []core.Container) []core.Container {
	noContainers := 4
	if f.cluster.Spec.QueryLimits != nil {
		noContainers += 1
	}

	if len(in) != noContainers {
		in = make([]core.Container, noContainers)
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

	// initialDelaySeconds = 30
	// timeoutSeconds = 5
	// periodSeconds = 5
	mysql.LivenessProbe = ensureProbe(mysql.LivenessProbe, 30, 5, 5, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				"--defaults-file=/etc/mysql/client.cnf",
				"ping",
			},
		},
	})

	// initialDelaySeconds = 30
	// timeoutSeconds = 2
	// periodSeconds = 2
	// we have to know ASAP when server is not ready to remove it from endpoints
	mysql.ReadinessProbe = ensureProbe(mysql.ReadinessProbe, 30, 2, 2, core.Handler{
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

	helper := f.ensureContainer(in[1], containerHelperName,
		f.cluster.Spec.GetHelperImage(),
		[]string{"config-and-serve"},
	)
	helper.Ports = ensureContainerPorts(helper.Ports, core.ContainerPort{
		Name:          HelperXtrabackupPortName,
		ContainerPort: HelperXtrabackupPort,
	})

	// HELPER container
	helper.ReadinessProbe = ensureProbe(helper.ReadinessProbe, 30, 5, 5, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   HelperServerProbePath,
			Port:   intstr.FromInt(HelperServerPort),
			Scheme: core.URISchemeHTTP,
		},
	})
	in[1] = helper

	// METRICS container
	exporter := f.ensureContainer(in[2], containerExporterName,
		f.cluster.Spec.GetMetricsExporterImage(),
		[]string{
			fmt.Sprintf("--web.listen-address=0.0.0.0:%d", ExporterPort),
			fmt.Sprintf("--web.telemetry-path=%s", ExporterPath),
			"--collect.heartbeat",
			fmt.Sprintf("--collect.heartbeat.database=%s", HelperDbName),
		},
	)
	exporter.Ports = ensureContainerPorts(mysql.Ports, core.ContainerPort{
		Name:          ExporterPortName,
		ContainerPort: ExporterPort,
	})
	exporter.LivenessProbe = ensureProbe(exporter.LivenessProbe, 30, 30, 30, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   ExporterPath,
			Port:   ExporterTargetPort,
			Scheme: core.URISchemeHTTP,
		},
	})

	in[2] = exporter

	// PT-HEARTBEAT container
	heartbeat := f.ensureContainer(in[3], containerHeartBeatName,
		f.cluster.Spec.GetHelperImage(),
		[]string{
			"pt-heartbeat",
			"--update", "--replace",
			"--check-read-only",
			"--create-table",
			"--database", HelperDbName,
			"--table", "heartbeat",
			"--defaults-file", fmt.Sprintf("%s/client.cnf", ConfVolumeMountPath),
		},
	)

	in[3] = heartbeat

	if f.cluster.Spec.QueryLimits != nil {
		command := []string{
			"pt-kill",
			// host need to be specified, see pt-kill bug: https://jira.percona.com/browse/PT-1223
			"--host=127.0.0.1",
			fmt.Sprintf("--defaults-file=%s/client.cnf", ConfVolumeMountPath),
		}
		command = append(command, f.cluster.Spec.QueryLimits.GetOptions()...)

		killer := f.ensureContainer(in[4], containerKillerName,
			f.cluster.Spec.GetHelperImage(),
			command,
		)
		in[4] = killer
	}

	return in
}

func (f *cFactory) ensureVolumes(in []core.Volume) []core.Volume {
	noVolumes := 3
	if len(in) != noVolumes {
		in = make([]core.Volume, noVolumes)
	}

	in[0] = ensureVolume(in[0], confVolumeName, core.VolumeSource{
		EmptyDir: &core.EmptyDirVolumeSource{},
	})

	fileMode := int32(0644)
	in[1] = ensureVolume(in[1], confMapVolumeName, core.VolumeSource{
		ConfigMap: &core.ConfigMapVolumeSource{
			LocalObjectReference: core.LocalObjectReference{
				Name: f.cluster.GetNameForResource(api.ConfigMap),
			},
			DefaultMode: &fileMode,
		},
	})

	in[2] = ensureVolume(in[2], dataVolumeName, core.VolumeSource{
		PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
			ClaimName: dataVolumeName,
		},
	})

	return in
}

func (f *cFactory) ensureVolumeClaimTemplates(in []core.PersistentVolumeClaim) []core.PersistentVolumeClaim {
	initPvc := false
	if len(in) == 0 {
		in = make([]core.PersistentVolumeClaim, 1)
		initPvc = true
	}
	data := in[0]

	data.Name = dataVolumeName

	if initPvc {
		// This can be set only when creating new PVC. It ensures that PVC can be
		// terminated after deleting parent MySQL cluster
		data.ObjectMeta.OwnerReferences = f.getOwnerReferences()
	}

	data.Spec = f.cluster.Spec.VolumeSpec.PersistentVolumeClaimSpec

	in[0] = data

	return in
}

func (f *cFactory) getEnvSourcesFor(name string) (envSources []core.EnvFromSource) {
	if name == containerCloneName && len(f.cluster.Spec.InitBucketSecretName) > 0 {
		envSources = append(envSources, core.EnvFromSource{
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: f.cluster.Spec.InitBucketSecretName,
				},
			},
		})
	}
	if name == containerHelperName {
		envSources = append(envSources, core.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: f.cluster.Spec.SecretName,
				},
			},
		})
	}
	return
}

func (f *cFactory) getVolumeMountsFor(name string) []core.VolumeMount {
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

	case containerCloneName, containerMysqlName, containerHelperName:
		return []core.VolumeMount{
			core.VolumeMount{
				Name:      confVolumeName,
				MountPath: ConfVolumeMountPath,
			},
			core.VolumeMount{
				Name:      dataVolumeName,
				MountPath: DataVolumeMountPath,
			},
		}

	case containerHeartBeatName, containerKillerName:
		return []core.VolumeMount{
			core.VolumeMount{
				Name:      confVolumeName,
				MountPath: ConfVolumeMountPath,
			},
		}
	}

	return nil
}

func ensureVolume(in core.Volume, name string, source core.VolumeSource) core.Volume {
	in.Name = name
	in.VolumeSource = source

	return in
}
