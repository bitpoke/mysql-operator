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
	"strings"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	clusterwrap "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/controller/mysqlcluster/internal/syncer"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	confVolumeName = "conf"
	// ConfVolumeMountPath is the path where mysql configs will be mounted
	ConfVolumeMountPath = "/etc/mysql"
	// ConfDPath is the path to extra mysql configs dir
	ConfDPath = "/etc/mysql/conf.d"

	confMapVolumeName = "config-map"
	// ConfMapVolumeMountPath represents the temp config mount path in init containers
	ConfMapVolumeMountPath = "/mnt/conf"

	dataVolumeName = "data"
	// DataVolumeMountPath is the path to mysql data
	DataVolumeMountPath = "/var/lib/mysql"
)

type sfsSyncer struct {
	cluster           *clusterwrap.MysqlCluster
	configMapRevision string
	secretRevision    string
	opt               *options.Options
}

// NewStatefulSetSyncer returns a syncer for stateful set
func NewStatefulSetSyncer(cluster *api.MysqlCluster, cmRev, secRev string, opt *options.Options) syncer.Interface {
	return &sfsSyncer{
		cluster:           clusterwrap.NewMysqlClusterWrapper(cluster),
		configMapRevision: cmRev,
		secretRevision:    secRev,
		opt:               opt,
	}
}

func (s *sfsSyncer) GetExistingObjectPlaceholder() runtime.Object {
	return &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.cluster.GetNameForResource(api.StatefulSet),
			Namespace: s.cluster.Namespace,
		},
	}
}

func (s *sfsSyncer) ShouldHaveOwnerReference() bool {
	return true
}

func (s *sfsSyncer) Sync(in runtime.Object) error {
	out := in.(*apps.StatefulSet)

	if out.Status.ReadyReplicas == s.cluster.Spec.Replicas {
		s.cluster.UpdateStatusCondition(api.ClusterConditionReady,
			core.ConditionTrue, "statefulset ready", "Cluster is ready.")
	} else {
		s.cluster.UpdateStatusCondition(api.ClusterConditionReady,
			core.ConditionFalse, "statefulset not ready", "Cluster is not ready.")
	}

	s.cluster.Status.ReadyNodes = int(out.Status.ReadyReplicas)

	out.Spec.Replicas = &s.cluster.Spec.Replicas
	out.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: s.getLabels(map[string]string{}),
	}

	out.Spec.ServiceName = s.cluster.GetNameForResource(api.HeadlessSVC)
	out.Spec.Template = s.ensureTemplate(out.Spec.Template)
	out.Spec.VolumeClaimTemplates = s.ensureVolumeClaimTemplates(out.Spec.VolumeClaimTemplates)

	return nil
}

func (s *sfsSyncer) ensureTemplate(in core.PodTemplateSpec) core.PodTemplateSpec {
	in.ObjectMeta.Labels = s.getLabels(s.cluster.Spec.PodSpec.Labels)
	in.ObjectMeta.Annotations = s.cluster.Spec.PodSpec.Annotations
	if len(in.ObjectMeta.Annotations) == 0 {
		in.ObjectMeta.Annotations = make(map[string]string)
	}
	in.ObjectMeta.Annotations["config_rev"] = s.configMapRevision
	in.ObjectMeta.Annotations["secret_rev"] = s.secretRevision
	in.ObjectMeta.Annotations["prometheus.io/scrape"] = "true"
	in.ObjectMeta.Annotations["prometheus.io/port"] = fmt.Sprintf("%d", api.ExporterPort)

	in.Spec.InitContainers = s.ensureInitContainersSpec(in.Spec.InitContainers)
	in.Spec.Containers = s.ensureContainersSpec(in.Spec.Containers)

	in.Spec.Volumes = s.ensureVolumes(in.Spec.Volumes)

	in.Spec.Affinity = &s.cluster.Spec.PodSpec.Affinity
	in.Spec.NodeSelector = s.cluster.Spec.PodSpec.NodeSelector
	in.Spec.ImagePullSecrets = s.cluster.Spec.PodSpec.ImagePullSecrets

	return in
}

const (
	containerInitName      = "init-mysql"
	containerCloneName     = "clone-mysql"
	containerSidecarName   = "mysql-operator-sidecar"
	containerMysqlName     = "mysql"
	containerExporterName  = "metrics-exporter"
	containerHeartBeatName = "pt-heartbeat"
	containerKillerName    = "pt-kill"
)

func (s *sfsSyncer) ensureContainer(in core.Container, name, image string, args []string) core.Container {
	in.Name = name
	in.Image = image
	in.ImagePullPolicy = s.cluster.Spec.PodSpec.ImagePullPolicy
	in.Args = args
	in.EnvFrom = s.getEnvSourcesFor(name)
	in.Env = s.getEnvFor(name)
	in.VolumeMounts = s.getVolumeMountsFor(name)

	return in
}

func (s *sfsSyncer) getEnvFor(name string) []core.EnvVar {
	boolTrue := true
	env := []core.EnvVar{}

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
		Value: s.cluster.GetNameForResource(api.HeadlessSVC),
	})
	env = append(env, core.EnvVar{
		Name:  "MY_CLUSTER_NAME",
		Value: s.cluster.Name,
	})
	env = append(env, core.EnvVar{
		Name:  "MY_FQDN",
		Value: "$(MY_POD_NAME).$(MY_SERVICE_NAME).$(MY_NAMESPACE)",
	})
	env = append(env, core.EnvVar{
		Name:  "ORCHESTRATOR_URI",
		Value: s.opt.OrchestratorURI,
	})

	if len(s.cluster.Spec.InitBucketURI) > 0 && name == containerCloneName {
		env = append(env, core.EnvVar{
			Name:  "INIT_BUCKET_URI",
			Value: s.cluster.Spec.InitBucketURI,
		})
	}

	switch name {
	case containerExporterName:
		env = append(env, core.EnvVar{
			Name: "USER",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
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
						Name: s.cluster.Spec.SecretName,
					},
					Key:      "BACKUP_PASSWORD",
					Optional: &boolTrue,
				},
			},
		})
	}

	return env
}

func (s *sfsSyncer) ensureInitContainersSpec(in []core.Container) []core.Container {
	if len(in) == 0 {
		in = make([]core.Container, 2)
	}

	// init container for configs
	in[0] = s.ensureContainer(in[0], containerInitName,
		s.opt.HelperImage,
		[]string{"files-config"},
	)

	// clone container
	in[1] = s.ensureContainer(in[1], containerCloneName,
		s.opt.HelperImage,
		[]string{"clone"},
	)

	return in
}

func (s *sfsSyncer) ensureContainersSpec(in []core.Container) []core.Container {
	noContainers := 4
	if s.cluster.Spec.QueryLimits != nil {
		noContainers++
	}

	if len(in) != noContainers {
		in = make([]core.Container, noContainers)
	}

	// MYSQL container
	mysql := s.ensureContainer(in[0], containerMysqlName,
		s.opt.MysqlImage+":"+s.opt.MysqlImageTag,
		[]string{},
	)
	mysql.Ports = ensureContainerPorts(mysql.Ports, core.ContainerPort{
		Name:          MysqlPortName,
		ContainerPort: MysqlPort,
	})
	mysql.Resources = s.cluster.Spec.PodSpec.Resources
	mysql.LivenessProbe = ensureProbe(mysql.LivenessProbe, 30, 5, 5, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				"--defaults-file=/etc/mysql/client.cnf",
				"ping",
			},
		},
	})

	// we have to know ASAP when server is not ready to remove it from endpoints
	mysql.ReadinessProbe = ensureProbe(mysql.ReadinessProbe, 5, 5, 2, core.Handler{
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

	helper := s.ensureContainer(in[1], containerSidecarName,
		s.opt.HelperImage,
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
	exporter := s.ensureContainer(in[2], containerExporterName,
		s.opt.MetricsExporterImage,
		[]string{
			fmt.Sprintf("--web.listen-address=0.0.0.0:%d", api.ExporterPort),
			fmt.Sprintf("--web.telemetry-path=%s", ExporterPath),
			"--collect.heartbeat",
			fmt.Sprintf("--collect.heartbeat.database=%s", HelperDbName),
		},
	)
	exporter.Ports = ensureContainerPorts(mysql.Ports, core.ContainerPort{
		Name:          ExporterPortName,
		ContainerPort: api.ExporterPort,
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
	heartbeat := s.ensureContainer(in[3], containerHeartBeatName,
		s.opt.HelperImage,
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

	if s.cluster.Spec.QueryLimits != nil {
		command := []string{
			"pt-kill",
			// host need to be specified, see pt-kill bug: https://jira.percona.com/browse/PT-1223
			"--host=127.0.0.1",
			fmt.Sprintf("--defaults-file=%s/client.cnf", ConfVolumeMountPath),
		}
		command = append(command, getCliOptionsFromQueryLimits(s.cluster.Spec.QueryLimits)...)

		killer := s.ensureContainer(in[4], containerKillerName,
			s.opt.HelperImage,
			command,
		)
		in[4] = killer
	}

	return in
}

func (s *sfsSyncer) ensureVolumes(in []core.Volume) []core.Volume {
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
				Name: s.cluster.GetNameForResource(api.ConfigMap),
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

// TODO rework the condition here after kubernetes-sigs/controller-runtime#98 gets merged.
//     we should be able to check s.cluster.ObjectMeta.CreationTimestamp.IsZero()

func (s *sfsSyncer) ensureVolumeClaimTemplates(in []core.PersistentVolumeClaim) []core.PersistentVolumeClaim {
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
		trueVar := true

		data.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			metav1.OwnerReference{
				APIVersion: api.SchemeGroupVersion.String(),
				Kind:       "MysqlCluster",
				Name:       s.cluster.Name,
				UID:        s.cluster.UID,
				Controller: &trueVar,
			},
		}
	}

	data.Spec = s.cluster.Spec.VolumeSpec.PersistentVolumeClaimSpec

	in[0] = data

	return in
}

func (s *sfsSyncer) getEnvSourcesFor(name string) []core.EnvFromSource {
	envSources := []core.EnvFromSource{}
	if name == containerCloneName && len(s.cluster.Spec.InitBucketSecretName) > 0 {
		envSources = append(envSources, core.EnvFromSource{
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: s.cluster.Spec.InitBucketSecretName,
				},
			},
		})
	}
	if name == containerSidecarName {
		envSources = append(envSources, core.EnvFromSource{
			Prefix: "MYSQL_",
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: s.cluster.Spec.SecretName,
				},
			},
		})
	}
	return envSources
}

func (s *sfsSyncer) getVolumeMountsFor(name string) []core.VolumeMount {
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

	case containerCloneName, containerMysqlName, containerSidecarName:
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

func (s *sfsSyncer) getLabels(extra map[string]string) map[string]string {
	defaultsLabels := s.cluster.GetLabels()
	for k, v := range extra {
		defaultsLabels[k] = v
	}
	return defaultsLabels
}

func getCliOptionsFromQueryLimits(ql *api.QueryLimits) []string {
	options := []string{
		"--print",
		// The purpose of this is to give blocked queries a chance to execute,
		// so we don’t kill a query that’s blocking a bunch of others, and then
		// kill the others immediately afterwards.
		"--wait-after-kill=1",
		"--busy-time", fmt.Sprintf("%d", ql.MaxQueryTime),
	}

	switch ql.KillMode {
	case "connection":
		options = append(options, "--kill")
	case "query":
		options = append(options, "--kill-query")
	default:
		options = append(options, "--kill-query")
	}

	if ql.MaxIdleTime != nil {
		options = append(options, "--idle-time", fmt.Sprintf("%d", *ql.MaxIdleTime))
	}

	if len(ql.Kill) != 0 {
		options = append(options, "--victims", ql.Kill)
	}

	if len(ql.IgnoreDb) > 0 {
		options = append(options, "--ignore-db", strings.Join(ql.IgnoreDb, "|"))
	}

	if len(ql.IgnoreCommand) > 0 {
		options = append(options, "--ignore-command", strings.Join(ql.IgnoreCommand, "|"))
	}

	if len(ql.IgnoreUser) > 0 {
		options = append(options, "--ignore-user", strings.Join(ql.IgnoreUser, "|"))
	}

	return options
}

func ensureProbe(in *core.Probe, delay, timeout, period int32, handler core.Handler) *core.Probe {
	if in == nil {
		in = &core.Probe{}
	}
	in.InitialDelaySeconds = delay
	in.TimeoutSeconds = timeout
	in.PeriodSeconds = period
	if handler.Exec != nil {
		in.Handler.Exec = handler.Exec
	}
	if handler.HTTPGet != nil {
		in.Handler.HTTPGet = handler.HTTPGet
	}
	if handler.TCPSocket != nil {
		in.Handler.TCPSocket = handler.TCPSocket
	}

	return in
}

func ensureContainerPorts(in []core.ContainerPort, ports ...core.ContainerPort) []core.ContainerPort {
	if len(in) == 0 {
		return ports
	}
	return in
}
