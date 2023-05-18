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
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/imdario/mergo"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/mergo/transformers"
	"github.com/presslabs/controller-util/syncer"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

// volumes names
const (
	confVolumeName    = "conf"
	confMapVolumeName = "config-map"
	dataVolumeName    = "data"
	tmpfsVolumeName   = "tmp"
)

// containers names
const (
	// init containers
	containerCloneAndInitName = "init"
	containerMySQLInitName    = "mysql-init-only"

	// containers
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

	return syncer.NewObjectSyncer("StatefulSet", cluster.Unwrap(), obj, c, func() error {
		return sync.SyncFn(obj)
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

	// mergo will add new keys for NodeSelector and Tolerations and keep the others instead of removing them
	// Fixes: https://github.com/bitpoke/mysql-operator/issues/454
	out.Spec.Template.Spec.NodeSelector = s.cluster.Spec.PodSpec.NodeSelector
	out.Spec.Template.Spec.Tolerations = s.cluster.Spec.PodSpec.Tolerations

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

// nolint: gocyclo
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
		Name:  "MY_MYSQL_VERSION",
		Value: s.cluster.GetMySQLSemVer().String(),
	})

	if len(s.cluster.Spec.InitBucketURL) > 0 && isCloneAndInit(name) {
		env = append(env, core.EnvVar{
			Name:  "INIT_BUCKET_URI",
			Value: s.cluster.Spec.InitBucketURL,
		})
	}

	if s.cluster.Spec.ServerIDOffset != nil {
		env = append(env, core.EnvVar{
			Name:  "MY_SERVER_ID_OFFSET",
			Value: strconv.FormatInt(int64(*s.cluster.Spec.ServerIDOffset), 10),
		})
	}

	hasBackupCompressCommand := len(s.cluster.Spec.BackupCompressCommand) > 0
	hasBackupDecompressCommand := len(s.cluster.Spec.BackupDecompressCommand) > 0
	if hasBackupCompressCommand && hasBackupDecompressCommand && isCloneAndInit(name) {
		env = append(env, core.EnvVar{
			Name:  "BACKUP_DECOMPRESS_COMMAND",
			Value: strings.Join(s.cluster.Spec.BackupDecompressCommand, " "),
		})
	} else if hasBackupDecompressCommand {
		log.Info("backupCompressCommand is not defined, falling back to gzip")
	}

	hasRcloneExtraArgs := len(s.cluster.Spec.RcloneExtraArgs) > 0
	if hasRcloneExtraArgs && (isCloneAndInit(name) || isSidecar(name)) {
		env = append(env, core.EnvVar{
			Name:  "RCLONE_EXTRA_ARGS",
			Value: strings.Join(s.cluster.Spec.RcloneExtraArgs, " "),
		})
	}

	hasXbstreamExtraArgs := len(s.cluster.Spec.XbstreamExtraArgs) > 0
	if hasXbstreamExtraArgs && (isCloneAndInit(name) || isSidecar(name)) {
		env = append(env, core.EnvVar{
			Name:  "XBSTREAM_EXTRA_ARGS",
			Value: strings.Join(s.cluster.Spec.XbstreamExtraArgs, " "),
		})
	}

	hasXtrabackupExtraArgs := len(s.cluster.Spec.XtrabackupExtraArgs) > 0
	if hasXtrabackupExtraArgs && isSidecar(name) {
		env = append(env, core.EnvVar{
			Name:  "XTRABACKUP_EXTRA_ARGS",
			Value: strings.Join(s.cluster.Spec.XtrabackupExtraArgs, " "),
		})
	}

	hasXtrabackupPrepareExtraArgs := len(s.cluster.Spec.XtrabackupPrepareExtraArgs) > 0
	if hasXtrabackupPrepareExtraArgs && isCloneAndInit(name) {
		env = append(env, core.EnvVar{
			Name:  "XTRABACKUP_PREPARE_EXTRA_ARGS",
			Value: strings.Join(s.cluster.Spec.XtrabackupPrepareExtraArgs, " "),
		})
	}

	hasXtrabackupTargetDir := len(s.cluster.Spec.XtrabackupTargetDir) > 0
	if hasXtrabackupTargetDir && isSidecar(name) {
		env = append(env, core.EnvVar{
			Name:  "XTRABACKUP_TARGET_DIR",
			Value: s.cluster.Spec.XtrabackupTargetDir,
		})
	}

	hasInitFileExtraSQL := len(s.cluster.Spec.InitFileExtraSQL) > 0
	if hasInitFileExtraSQL && isCloneAndInit(name) {
		env = append(env, core.EnvVar{
			Name:  "INITFILE_EXTRA_SQL",
			Value: strings.Join(s.cluster.Spec.InitFileExtraSQL, ";"),
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
			Value: fmt.Sprintf("$(USER):$(PASSWORD)@(127.0.0.1:%d)/", s.cluster.ExporterDataSourcePort()),
		})
	case containerMySQLInitName:
		// set MySQL init only flag for init container
		env = append(env, core.EnvVar{
			Name:  "MYSQL_INIT_ONLY",
			Value: "1",
		})
	case containerCloneAndInitName:
		env = append(env, s.envVarFromSecret(sctOpName, "BACKUP_USER", "BACKUP_USER", true))
		env = append(env, s.envVarFromSecret(sctOpName, "BACKUP_PASSWORD", "BACKUP_PASSWORD", true))
	case containerMysqlName:
		env = append(env, core.EnvVar{
			Name:  "ORCH_CLUSTER_ALIAS",
			Value: s.cluster.GetClusterAlias(),
		})
		env = append(env, core.EnvVar{
			Name:  "ORCH_HTTP_API",
			Value: s.opt.OrchestratorURI,
		})
	}

	// set MySQL root and application credentials
	if name == containerMySQLInitName || (!s.cluster.ShouldHaveInitContainerForMysql() && name == containerMysqlName) {
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_ROOT_PASSWORD", "ROOT_PASSWORD", false))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_USER", "USER", true))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_PASSWORD", "PASSWORD", true))
		env = append(env, s.envVarFromSecret(sctName, "MYSQL_DATABASE", "DATABASE", true))
	}

	return env
}

func (s *sfsSyncer) ensureInitContainersSpec() []core.Container {
	initCs := []core.Container{}

	// add user defined init containers
	if len(s.cluster.Spec.PodSpec.InitContainers) > 0 {
		initCs = append(initCs, s.cluster.Spec.PodSpec.InitContainers...)
	}

	// clone and init container
	cloneInit := s.ensureContainer(containerCloneAndInitName,
		s.cluster.GetSidecarImage(),
		[]string{"clone-and-init"},
	)
	initCs = append(initCs, cloneInit)

	// add init container for MySQL if docker image supports this
	if s.cluster.ShouldHaveInitContainerForMysql() {
		mysqlInit := s.ensureContainer(containerMySQLInitName,
			s.cluster.GetMysqlImage(),
			[]string{})
		mysqlInit.Resources = s.ensureResources(containerMySQLInitName)
		initCs = append(initCs, mysqlInit)
	}

	return initCs
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
	mysql.Resources = s.ensureResources(containerMysqlName)
	mysql.LivenessProbe = ensureProbe(60, 5, 5, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"mysqladmin",
				fmt.Sprintf("--defaults-file=%s", confClientPath),
				"ping",
			},
		},
	})

	// set lifecycle hook on MySQL container
	if s.cluster.Spec.PodSpec.MysqlLifecycle != nil {
		mysql.Lifecycle = s.cluster.Spec.PodSpec.MysqlLifecycle
	} else if s.opt.FailoverBeforeShutdownEnabled {
		mysql.Lifecycle = &core.Lifecycle{
			PreStop: &core.Handler{
				Exec: &core.ExecAction{
					Command: []string{"bash", fmt.Sprintf("%s/%s", ConfVolumeMountPath, shPreStopFile)},
				},
			},
		}
	}

	// nolint: gosec
	mysqlTestCmd := fmt.Sprintf(`mysql --defaults-file=%s -NB -e 'SELECT COUNT(*) FROM %s.%s WHERE name="configured" AND value="1"'`,
		confClientPath, constants.OperatorDbName, constants.OperatorStatusTableName)

	// we have to know ASAP when server is not ready to remove it from endpoints
	mysql.ReadinessProbe = ensureProbe(5, 5, 2, core.Handler{
		Exec: &core.ExecAction{
			Command: []string{
				"/bin/sh", "-c",
				fmt.Sprintf(`test $(%s) -eq 1`, mysqlTestCmd),
			},
		},
	})

	// SIDECAR container
	sidecar := s.ensureContainer(containerSidecarName,
		s.cluster.GetSidecarImage(),
		[]string{"config-and-serve"},
	)
	sidecar.Ports = ensurePorts(core.ContainerPort{
		Name:          SidecarServerPortName,
		ContainerPort: SidecarServerPort,
	})
	sidecar.Resources = s.ensureResources(containerSidecarName)
	sidecar.ReadinessProbe = ensureProbe(30, 5, 5, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   SidecarServerProbePath,
			Port:   intstr.FromInt(SidecarServerPort),
			Scheme: core.URISchemeHTTP,
		},
	})

	// METRICS container
	exporterCommand := []string{
		fmt.Sprintf("--web.listen-address=0.0.0.0:%d", ExporterPort),
		fmt.Sprintf("--web.telemetry-path=%s", ExporterPath),
		"--collect.heartbeat",
		fmt.Sprintf("--collect.heartbeat.database=%s", constants.OperatorDbName),
	}

	if len(s.cluster.Spec.MetricsExporterExtraArgs) > 0 {
		exporterCommand = append(exporterCommand, s.cluster.Spec.MetricsExporterExtraArgs...)
	}

	exporter := s.ensureContainer(containerExporterName,
		s.opt.MetricsExporterImage,
		exporterCommand,
	)

	exporter.Ports = ensurePorts(core.ContainerPort{
		Name:          ExporterPortName,
		ContainerPort: ExporterPort,
	})

	exporter.Resources = s.ensureResources(containerExporterName)

	exporter.LivenessProbe = ensureProbe(30, 30, 30, core.Handler{
		HTTPGet: &core.HTTPGetAction{
			Path:   ExporterPath,
			Port:   ExporterTargetPort,
			Scheme: core.URISchemeHTTP,
		},
	})

	// PT-HEARTBEAT container
	heartbeat := s.ensureContainer(containerHeartBeatName,
		s.cluster.GetSidecarImage(),
		[]string{
			"pt-heartbeat",
			"--update", "--replace",
			"--check-read-only",
			"--create-table",
			"--database", constants.OperatorDbName,
			"--table", "heartbeat",
			"--utc",
			"--defaults-file", constants.ConfHeartBeatPath,
			// it's important to exit when exceeding more than 20 failed attempts otherwise
			// pt-heartbeat will run forever using old connection.
			"--fail-successive-errors=20",
		},
	)
	heartbeat.Resources = s.ensureResources(containerHeartBeatName)

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
			fmt.Sprintf("--defaults-file=%s/client.conf", ConfVolumeMountPath),
		}
		command = append(command, getCliOptionsFromQueryLimits(s.cluster.Spec.QueryLimits)...)

		killer := s.ensureContainer(containerKillerName,
			s.cluster.GetSidecarImage(),
			command,
		)

		killer.Resources = s.ensureResources(containerKillerName)

		containers = append(containers, killer)
	}

	// add user defined containers
	if len(s.cluster.Spec.PodSpec.Containers) > 0 {
		containers = append(containers, s.cluster.Spec.PodSpec.Containers...)
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
		log.Info("no an allowed volume spec is specified", "volumeSpec", s.cluster.Spec.VolumeSpec, "key", s.cluster)
	}

	volumes := []core.Volume{
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

		ensureVolume(dataVolumeName, dataVolume),
	}

	if s.cluster.Spec.TmpfsSize != nil {
		volumes = append(volumes, ensureVolume(tmpfsVolumeName, core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{
				Medium:    core.StorageMediumMemory,
				SizeLimit: s.cluster.Spec.TmpfsSize,
			},
		}))
	}

	// append the custom volumes defined by the user
	if len(s.cluster.Spec.PodSpec.Volumes) > 0 {
		volumes = append(volumes, s.cluster.Spec.PodSpec.Volumes...)
	}

	return volumes
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
	if isCloneAndInit(name) && len(s.cluster.Spec.InitBucketSecretName) > 0 {
		envSources = append(envSources, core.EnvFromSource{
			SecretRef: &core.SecretEnvSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: s.cluster.Spec.InitBucketSecretName,
				},
			},
		})
	}
	if isCloneAndInit(name) || isSidecar(name) {
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
	case containerCloneAndInitName:
		mounts := []core.VolumeMount{
			{Name: confVolumeName, MountPath: ConfVolumeMountPath},
			{Name: confMapVolumeName, MountPath: ConfMapVolumeMountPath},
			{Name: dataVolumeName, MountPath: DataVolumeMountPath},
		}

		return mounts

	case containerMysqlName, containerSidecarName, containerMySQLInitName:
		mounts := []core.VolumeMount{
			{Name: confVolumeName, MountPath: ConfVolumeMountPath},
			{Name: dataVolumeName, MountPath: DataVolumeMountPath},
		}
		if s.cluster.Spec.TmpfsSize != nil {
			mounts = append(mounts, core.VolumeMount{Name: tmpfsVolumeName, MountPath: DataVolumeMountPath})
		}

		// add custom volume mounts to the mysql containers
		if len(s.cluster.Spec.PodSpec.VolumeMounts) > 0 {
			mounts = append(mounts, s.cluster.Spec.PodSpec.VolumeMounts...)
		}

		return mounts

	case containerHeartBeatName, containerKillerName:
		return []core.VolumeMount{
			{Name: confVolumeName, MountPath: ConfVolumeMountPath},
		}
	}

	return nil
}

func (s *sfsSyncer) getLabels(extra map[string]string) map[string]string {
	defaultsLabels := s.cluster.GetLabels()
	for k, v := range extra {
		defaultsLabels[k] = v
	}
	return defaultsLabels
}

func (s *sfsSyncer) ensureResources(name string) core.ResourceRequirements {
	limits := core.ResourceList{
		core.ResourceCPU: resource.MustParse("100m"),
	}
	requests := core.ResourceList{
		core.ResourceCPU:    resource.MustParse("10m"),
		core.ResourceMemory: resource.MustParse("32Mi"),
	}

	switch name {
	case containerExporterName:
		return s.cluster.Spec.PodSpec.MetricsExporterResources

	case containerMySQLInitName, containerMysqlName:
		return s.cluster.Spec.PodSpec.Resources

	case containerHeartBeatName:
		limits[core.ResourceMemory] = resource.MustParse("64Mi")

	case containerSidecarName:
		return s.cluster.Spec.PodSpec.MySQLOperatorSidecarResources

	}

	return core.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}

func ensureVolume(name string, source core.VolumeSource) core.Volume {
	return core.Volume{
		Name:         name,
		VolumeSource: source,
	}
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

func isCloneAndInit(name string) bool {
	return name == containerCloneAndInitName
}

func isSidecar(name string) bool {
	return name == containerSidecarName
}
