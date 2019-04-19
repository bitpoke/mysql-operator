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
	"github.com/presslabs/mysql-operator/pkg/util/constants"
	"strings"

	"github.com/imdario/mergo"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/mergo/transformers"
	"github.com/presslabs/controller-util/syncer"
	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

// volumes names
const (
	confVolumeName    = "conf"
	confMapVolumeName = "config-map"
	initDBVolumeName  = "init-scripts"
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
func NewStatefulSetSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, cmRev, sctRev string, opt *options.Options) syncer.Interface {
	obj := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
			Namespace: cluster.Namespace,
		},
	}

	sync := &sfsSyncer{
		cluster:           cluster,
		configMapRevision: cmRev,
		secretRevision:    sctRev,
		opt:               opt,
	}

	return syncer.NewObjectSyncer("StatefulSet", cluster.Unwrap(), obj, c, scheme, func(in runtime.Object) error {
		return sync.SyncFn(in)
	})
}

func (s *sfsSyncer) SyncFn(in runtime.Object) error {
	out := in.(*apps.StatefulSet)

	s.cluster.Status.ReadyNodes = int(out.Status.ReadyReplicas)

	out.Spec.Replicas = s.cluster.Spec.Replicas
	out.Spec.Selector = metav1.SetAsLabelSelector(s.cluster.GetSelectorLabels())
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

	if s.cluster.Spec.VolumeSpec.PersistentVolumeClaim != nil {
		out.Spec.VolumeClaimTemplates = s.ensureVolumeClaimTemplates(out.Spec.VolumeClaimTemplates)
	}

	return nil
}

func (s *sfsSyncer) ensurePodSpec() core.PodSpec {
	fsGroup := int64(999) // mysql user UID
	return core.PodSpec{
		InitContainers: s.ensureInitContainersSpec(),
		Containers:     s.ensureContainersSpec(),
		Volumes:        s.ensureVolumes(),
		SecurityContext: &core.PodSecurityContext{
			// mount volumes with mysql gid
			FSGroup:   &fsGroup,
			RunAsUser: &fsGroup,
		},
		Affinity:           s.cluster.Spec.PodSpec.Affinity,
		ImagePullSecrets:   s.cluster.Spec.PodSpec.ImagePullSecrets,
		NodeSelector:       s.cluster.Spec.PodSpec.NodeSelector,
		PriorityClassName:  s.cluster.Spec.PodSpec.PriorityClassName,
		Tolerations:        s.cluster.Spec.PodSpec.Tolerations,
		ServiceAccountName: s.cluster.Spec.PodSpec.ServiceAccountName,
		// TODO: uncomment this when limiting operator for k8s version > 1.13
		// ReadinessGates: []core.PodReadinessGate{
		// 	{
		// 		ConditionType: mysqlcluster.NodeInitializedConditionType,
		// 	},
		// },
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

func (s *sfsSyncer) envVarFromSecret(sctName, name, key string, opt bool) core.EnvVar {
	env := core.EnvVar{
		Name: name,
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: sctName,
				},
				Key:      key,
				Optional: &opt,
			},
		},
	}
	return env
}

func (s *sfsSyncer) getEnvFor(name string) []core.EnvVar {
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

	if len(s.cluster.Spec.InitBucketURL) > 0 && name == containerCloneName {
		env = append(env, core.EnvVar{
			Name:  "INIT_BUCKET_URI",
			Value: s.cluster.Spec.InitBucketURL,
		})
	}

	sctName := s.cluster.Spec.SecretName
	sctOpName := s.cluster.GetNameForResource(mysqlcluster.Secret)
	switch name {
	case containerExporterName:
		env = append(env, s.envVarFromSecret(sctOpName, "USER", "METRICS_EXPORTER_USER", false))
		env = append(env, s.envVarFromSecret(sctOpName, "PASSWORD", "METRICS_EXPORTER_PASSWORD", false))
		env = append(env, core.EnvVar{
			Name:  "DATA_SOURCE_NAME",
			Value: fmt.Sprintf("$(USER):$(PASSWORD)@(127.0.0.1:%d)/", MysqlPort),
		})
	case containerMysqlName:
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_ROOT_PASSWORD", "ROOT_PASSWORD", false))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_USER", "USER", true))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_PASSWORD", "PASSWORD", true))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_DATABASE", "DATABASE", true))
	case containerCloneName:
		env = append(env, s.envVarFromSecret(sctOpName, "BACKUP_USER", "BACKUP_USER", true))
		env = append(env, s.envVarFromSecret(sctOpName, "BACKUP_PASSWORD", "BACKUP_PASSWORD", true))
	}

	return env
}

func (s *sfsSyncer) ensureInitContainersSpec() []core.Container {
	return []core.Container{
		// init container for configs
		s.ensureContainer(containerInitName,
			s.opt.SidecarImage,
			[]string{"files-config"},
		),

		// clone container
		s.ensureContainer(containerCloneName,
			s.opt.SidecarImage,
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
	mysql.LivenessProbe = ensureProbe(60, 5, 5, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				fmt.Sprintf("--defaults-file=%s", confClientPath),
				"ping",
			},
		},
	})

	// we have to know ASAP when server is not ready to remove it from endpoints
	mysql.ReadinessProbe = ensureProbe(5, 5, 2, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysql",
				fmt.Sprintf("--defaults-file=%s", confClientPath),
				"-e",
				"SELECT 1",
			},
		},
	})

	// SIDECAR container
	sidecar := s.ensureContainer(containerSidecarName,
		s.opt.SidecarImage,
		[]string{"config-and-serve"},
	)
	sidecar.Ports = ensurePorts(core.ContainerPort{
		Name:          SidecarServerPortName,
		ContainerPort: SidecarServerPort,
	})
	sidecar.Resources = ensureResources(containerSidecarName)
	sidecar.ReadinessProbe = ensureProbe(30, 5, 5, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   SidecarServerProbePath,
			Port:   intstr.FromInt(SidecarServerPort),
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
			fmt.Sprintf("--collect.heartbeat.database=%s", constants.OperatorDbName),
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
		s.opt.SidecarImage,
		[]string{
			"pt-heartbeat",
			"--update", "--replace",
			"--check-read-only",
			"--create-table",
			"--database", constants.OperatorDbName,
			"--table", "heartbeat",
			"--defaults-file", constants.ConfHeartBeatPath,
			// it's important to exit when exceeding more than 20 failed attempts otherwise
			// pt-heartbeat will run forever using old connection.
			"--fail-successive-errors=20",
		},
	)
	heartbeat.Resources = ensureResources(containerHeartBeatName)

	containers := []core.Container{
		mysql,
		sidecar,
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
			s.opt.SidecarImage,
			command,
		)

		killer.Resources = ensureResources(containerKillerName)

		containers = append(containers, killer)
	}

	return containers

}

func (s *sfsSyncer) ensureVolumes() []core.Volume {
	fileMode := int32(0644)
	dataVolume := core.VolumeSource{}

	if s.cluster.Spec.VolumeSpec.PersistentVolumeClaim != nil {
		dataVolume.PersistentVolumeClaim = &core.PersistentVolumeClaimVolumeSource{
			ClaimName: dataVolumeName,
		}
	} else if s.cluster.Spec.VolumeSpec.HostPath != nil {
		dataVolume.HostPath = s.cluster.Spec.VolumeSpec.HostPath
	} else if s.cluster.Spec.VolumeSpec.EmptyDir != nil {
		dataVolume.EmptyDir = s.cluster.Spec.VolumeSpec.EmptyDir
	} else {
		log.Error(nil, "no volume spec is specified", ".spec.volumeSpec", s.cluster.Spec.VolumeSpec)
	}

	return []core.Volume{
		ensureVolume(confVolumeName, core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{},
		}),

		ensureVolume(initDBVolumeName, core.VolumeSource{
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

		ensureVolume(dataVolumeName, dataVolume),
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
			{
				APIVersion: api.SchemeGroupVersion.String(),
				Kind:       "MysqlCluster",
				Name:       s.cluster.Name,
				UID:        s.cluster.UID,
				Controller: &trueVar,
			},
		}
	}

	data.Spec = *s.cluster.Spec.VolumeSpec.PersistentVolumeClaim

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
	if name == containerSidecarName || name == containerInitName {
		envSources = append(envSources, core.EnvFromSource{
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: s.cluster.GetNameForResource(mysqlcluster.Secret),
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
			{
				Name:      confVolumeName,
				MountPath: ConfVolumeMountPath,
			},
			{
				Name:      confMapVolumeName,
				MountPath: ConfMapVolumeMountPath,
			},
			{
				Name:      initDBVolumeName,
				MountPath: constants.InitDBVolumeMountPath,
			},
		}

	case containerCloneName, containerMysqlName, containerSidecarName:
		return []core.VolumeMount{
			{
				Name:      confVolumeName,
				MountPath: ConfVolumeMountPath,
			},
			{
				Name:      dataVolumeName,
				MountPath: DataVolumeMountPath,
			},
			{
				Name:      initDBVolumeName,
				MountPath: constants.InitDBVolumeMountPath,
			},
		}

	case containerHeartBeatName, containerKillerName:
		return []core.VolumeMount{
			{
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
