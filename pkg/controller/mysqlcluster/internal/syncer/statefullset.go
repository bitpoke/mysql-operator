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

	"github.com/appscode/mergo"
	"github.com/presslabs/controller-util/mergo/transformers"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

// volumes names
const (
	confVolumeName    = "conf"
	confMapVolumeName = "config-map"
	dataVolumeName    = "data"
)

// containers names
const (
	containerInitName      = "init-mysql"
	containerCloneName     = "clone-mysql"
	containerSidecarName   = "sidecar"
	containerMysqlName     = "mysql"
	containerExporterName  = "metrics-exporter"
	containerHeartBeatName = "pt-heartbeat"
	containerKillerName    = "pt-kill"
)

type sfsSyncer struct {
	cluster           *mysqlcluster.MysqlCluster
	configMapRevision string
	secretRevision    string
	opt               *options.Options
}

// NewStatefulSetSyncer returns a syncer for stateful set
func NewStatefulSetSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, cmRev, secRev string, opt *options.Options) syncer.Interface {
	obj := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
			Namespace: cluster.Namespace,
		},
	}

	sync := &sfsSyncer{
		cluster:           cluster,
		configMapRevision: cmRev,
		secretRevision:    secRev,
		opt:               opt,
	}

	return syncer.NewObjectSyncer("StatefulSet", cluster.Unwrap(), obj, c, scheme, func(in runtime.Object) error {
		return sync.SyncFn(in)
	})
}

func (s *sfsSyncer) SyncFn(in runtime.Object) error {
	out := in.(*apps.StatefulSet)

	if out.Status.ReadyReplicas == *s.cluster.Spec.Replicas {
		s.cluster.UpdateStatusCondition(api.ClusterConditionReady,
			core.ConditionTrue, "statefulset ready", "Cluster is ready.")
	} else {
		s.cluster.UpdateStatusCondition(api.ClusterConditionReady,
			core.ConditionFalse, "statefulset not ready", "Cluster is not ready.")
	}

	s.cluster.Status.ReadyNodes = int(out.Status.ReadyReplicas)

	out.Spec.Replicas = s.cluster.Spec.Replicas
	out.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: s.getLabels(map[string]string{}),
	}

	out.Spec.ServiceName = s.cluster.GetNameForResource(mysqlcluster.HeadlessSVC)

	// ensure template
	out.Spec.Template.ObjectMeta.Labels = s.getLabels(s.cluster.Spec.PodSpec.Labels)
	out.Spec.Template.ObjectMeta.Annotations = s.cluster.Spec.PodSpec.Annotations
	if len(out.Spec.Template.ObjectMeta.Annotations) == 0 {
		out.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	out.Spec.Template.ObjectMeta.Annotations["config_rev"] = s.configMapRevision
	out.Spec.Template.ObjectMeta.Annotations["secret_rev"] = s.secretRevision
	out.Spec.Template.ObjectMeta.Annotations["prometheus.io/scrape"] = "true"
	out.Spec.Template.ObjectMeta.Annotations["prometheus.io/port"] = fmt.Sprintf("%d", ExporterPort)

	err := mergo.Merge(&out.Spec.Template.Spec, s.ensurePodSpec(), mergo.WithTransformers(transformers.PodSpec))
	if err != nil {
		return err
	}

	out.Spec.VolumeClaimTemplates = s.ensureVolumeClaimTemplates(out.Spec.VolumeClaimTemplates)

	return nil
}

func (s *sfsSyncer) ensurePodSpec() core.PodSpec {
	return core.PodSpec{
		InitContainers:   s.ensureInitContainersSpec(),
		Containers:       s.ensureContainersSpec(),
		Volumes:          s.ensureVolumes(),
		Affinity:         &s.cluster.Spec.PodSpec.Affinity,
		NodeSelector:     s.cluster.Spec.PodSpec.NodeSelector,
		ImagePullSecrets: s.cluster.Spec.PodSpec.ImagePullSecrets,
	}
}

func (s *sfsSyncer) ensureContainer(name, image string, args []string) core.Container {
	return core.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: s.cluster.Spec.PodSpec.ImagePullPolicy,
		Args:            args,
		EnvFrom:         s.getEnvSourcesFor(name),
		Env:             s.getEnvFor(name),
		VolumeMounts:    s.getVolumeMountsFor(name),
	}
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
		Value: s.cluster.GetNameForResource(mysqlcluster.HeadlessSVC),
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

func (s *sfsSyncer) ensureInitContainersSpec() []core.Container {
	return []core.Container{
		// init container for configs
		s.ensureContainer(containerInitName,
			s.opt.HelperImage,
			[]string{"files-config"},
		),

		// clone container
		s.ensureContainer(containerCloneName,
			s.opt.HelperImage,
			[]string{"clone"},
		),
	}
}

func (s *sfsSyncer) ensureContainersSpec() []core.Container {
	// MYSQL container
	mysql := s.ensureContainer(containerMysqlName,
		s.cluster.GetMysqlImage(),
		[]string{},
	)
	mysql.Ports = ensurePorts(core.ContainerPort{
		Name:          MysqlPortName,
		ContainerPort: MysqlPort,
	})
	mysql.Resources = s.cluster.Spec.PodSpec.Resources
	mysql.LivenessProbe = ensureProbe(30, 5, 5, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				"--defaults-file=/etc/mysql/client.cnf",
				"ping",
			},
		},
	})

	// we have to know ASAP when server is not ready to remove it from endpoints
	mysql.ReadinessProbe = ensureProbe(5, 5, 2, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysql",
				"--defaults-file=/etc/mysql/client.cnf",
				"-e",
				"SELECT 1",
			},
		},
	})

	helper := s.ensureContainer(containerSidecarName,
		s.opt.HelperImage,
		[]string{"config-and-serve"},
	)
	helper.Ports = ensurePorts(core.ContainerPort{
		Name:          HelperXtrabackupPortName,
		ContainerPort: HelperXtrabackupPort,
	})
	helper.Resources = ensureResources(containerSidecarName)

	// HELPER container
	helper.ReadinessProbe = ensureProbe(30, 5, 5, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   HelperServerProbePath,
			Port:   intstr.FromInt(HelperServerPort),
			Scheme: core.URISchemeHTTP,
		},
	})

	// METRICS container
	exporter := s.ensureContainer(containerExporterName,
		s.opt.MetricsExporterImage,
		[]string{
			fmt.Sprintf("--web.listen-address=0.0.0.0:%d", ExporterPort),
			fmt.Sprintf("--web.telemetry-path=%s", ExporterPath),
			"--collect.heartbeat",
			fmt.Sprintf("--collect.heartbeat.database=%s", HelperDbName),
		},
	)
	exporter.Ports = ensurePorts(core.ContainerPort{
		Name:          ExporterPortName,
		ContainerPort: ExporterPort,
	})

	exporter.Resources = ensureResources(containerExporterName)

	exporter.LivenessProbe = ensureProbe(30, 30, 30, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   ExporterPath,
			Port:   ExporterTargetPort,
			Scheme: core.URISchemeHTTP,
		},
	})

	// PT-HEARTBEAT container
	heartbeat := s.ensureContainer(containerHeartBeatName,
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
	heartbeat.Resources = ensureResources(containerHeartBeatName)

	containers := []core.Container{
		mysql,
		helper,
		exporter,
		heartbeat,
	}

	// PT-KILL container
	if s.cluster.Spec.QueryLimits != nil {
		command := []string{
			"pt-kill",
			// host need to be specified, see pt-kill bug: https://jira.percona.com/browse/PT-1223
			"--host=127.0.0.1",
			fmt.Sprintf("--defaults-file=%s/client.cnf", ConfVolumeMountPath),
		}
		command = append(command, getCliOptionsFromQueryLimits(s.cluster.Spec.QueryLimits)...)

		killer := s.ensureContainer(containerKillerName,
			s.opt.HelperImage,
			command,
		)

		killer.Resources = ensureResources(containerKillerName)

		containers = append(containers, killer)
	}

	return containers

}

func (s *sfsSyncer) ensureVolumes() []core.Volume {
	fileMode := int32(0644)
	return []core.Volume{
		ensureVolume(confVolumeName, core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{},
		}),

		ensureVolume(confMapVolumeName, core.VolumeSource{
			ConfigMap: &core.ConfigMapVolumeSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: s.cluster.GetNameForResource(mysqlcluster.ConfigMap),
				},
				DefaultMode: &fileMode,
			},
		}),

		ensureVolume(dataVolumeName, core.VolumeSource{
			PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
				ClaimName: dataVolumeName,
			},
		}),
	}
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

func ensureVolume(name string, source core.VolumeSource) core.Volume {
	return core.Volume{
		Name:         name,
		VolumeSource: source,
	}
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

func ensureProbe(delay, timeout, period int32, handler core.Handler) *core.Probe {
	return &core.Probe{
		InitialDelaySeconds: delay,
		TimeoutSeconds:      timeout,
		PeriodSeconds:       period,
		Handler:             handler,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
}

func ensurePorts(ports ...core.ContainerPort) []core.ContainerPort {
	return ports
}

func ensureResources(name string) core.ResourceRequirements {
	limits := core.ResourceList{
		core.ResourceCPU: resource.MustParse("50m"),
	}
	requests := core.ResourceList{
		core.ResourceCPU: resource.MustParse("10m"),
	}

	switch name {
	case containerExporterName:
		limits = core.ResourceList{
			core.ResourceCPU: resource.MustParse("100m"),
		}
	}

	return core.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}
