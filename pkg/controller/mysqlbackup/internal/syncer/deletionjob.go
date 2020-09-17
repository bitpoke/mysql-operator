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

package syncer

import (
	"fmt"
	"strings"

	"github.com/imdario/mergo"
	"github.com/presslabs/controller-util/mergo/transformers"
	"github.com/presslabs/controller-util/syncer"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

const (
	// RemoteStorageFinalizer is the finalizer name used when hardDelete policy is used
	RemoteStorageFinalizer = "backups.mysql.presslabs.org/remote-storage-cleanup"

	// RemoteDeletionFailedEvent is the event that is set on the cluster when the cleanup job fails
	RemoteDeletionFailedEvent = "RemoteDeletionFailed"
)

type deletionJobSyncer struct {
	backup   *mysqlbackup.MysqlBackup
	cluster  *mysqlcluster.MysqlCluster
	opt      *options.Options
	schema   *runtime.Scheme
	recorder record.EventRecorder
}

// NewDeleteJobSyncer returns a job syncer for hard deletion job. The job which removes the backup
// from remote storage.
func NewDeleteJobSyncer(c client.Client, s *runtime.Scheme, backup *mysqlbackup.MysqlBackup,
	cluster *mysqlcluster.MysqlCluster, opt *options.Options, r record.EventRecorder) syncer.Interface {

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.GetNameForDeletionJob(),
			Namespace: backup.Namespace,
		},
	}

	jobSyncer := deletionJobSyncer{
		cluster:  cluster,
		backup:   backup,
		opt:      opt,
		schema:   s,
		recorder: r,
	}

	return syncer.NewObjectSyncer("BackupCleaner", nil, job, c, s, func() error {
		return jobSyncer.SyncFn(job)
	})
}

// nolint: gocyclo
func (s *deletionJobSyncer) SyncFn(job *batch.Job) error {
	if s.backup.Spec.RemoteDeletePolicy == api.Retain {
		// do nothing
		return syncer.ErrIgnore
	}

	// it's hard delete policy then set finalizer on backup
	addFinalizer(s.backup.Unwrap(), RemoteStorageFinalizer)

	if s.backup.DeletionTimestamp == nil {
		// don't do anything if the backup is not deleted
		return syncer.ErrIgnore
	}

	// if the backup is failed then don't create the deletion job and remove the finalizer
	if cond := s.backup.GetBackupCondition(api.BackupFailed); cond != nil && cond.Status == core.ConditionTrue {
		removeFinalizer(s.backup.Unwrap(), RemoteStorageFinalizer)
		return syncer.ErrIgnore
	}

	if len(s.backup.Spec.BackupURL) == 0 {
		return fmt.Errorf("empty .spec.backupURL")
	}

	// check if the job is created and if not create it
	if job.ObjectMeta.CreationTimestamp.IsZero() {
		job.Labels = map[string]string{
			"backup":      s.backup.Name,
			"cleanup-job": "true",
		}

		err := mergo.Merge(&job.Spec.Template.Spec, s.ensurePodSpec(),
			mergo.WithTransformers(transformers.PodSpec))
		if err != nil {
			return err
		}

		// explicit set owner reference on job because  the owner has set deletionTimestamp, at this point, and
		// the syncer will not set it
		err = controllerutil.SetControllerReference(s.backup.Unwrap(), job, s.schema)
		if err != nil {
			return err
		}
	}

	completed, failed := getJobStatus(job)
	if completed {
		removeFinalizer(s.backup.Unwrap(), RemoteStorageFinalizer)
	}

	// announce the cluster if deletion from remote storage failed
	if failed {
		s.recordWEventOnCluster(RemoteDeletionFailedEvent, "job failed")
	}

	return nil
}

func (s *deletionJobSyncer) ensurePodSpec() core.PodSpec {
	// get the service account, the same as the one used by the cluster if the cluster exists otherwise use
	// the default one. This may cause some issues when using workload identity in case when the cluster is removed
	// before the backup.
	serviceAccountName := ""
	if s.cluster != nil {
		serviceAccountName = s.cluster.Spec.PodSpec.ServiceAccountName
	}

	return core.PodSpec{
		RestartPolicy: core.RestartPolicyNever,
		Containers:    s.ensureContainers(),
		ImagePullSecrets: []core.LocalObjectReference{
			{Name: s.opt.ImagePullSecretName},
		},
		// set service account to this pod in order to be able to connect to remote storage if using workload identity
		ServiceAccountName: serviceAccountName,
	}
}

func (s *deletionJobSyncer) ensureContainers() []core.Container {
	rcloneCommand := []string{"rclone", fmt.Sprintf("--config=%s", constants.RcloneConfigFile)}

	if s.cluster != nil && len(s.cluster.Spec.RcloneExtraArgs) > 0 {
		rcloneCommand = append(rcloneCommand, s.cluster.Spec.RcloneExtraArgs...)
	}

	rcloneCommand = append(rcloneCommand, "delete", bucketForRclone(s.backup.Spec.BackupURL))

	container := core.Container{
		Name:            "delete",
		Image:           s.opt.SidecarImage,
		ImagePullPolicy: s.opt.ImagePullPolicy,
		Args:            rcloneCommand,
	}

	// if backups secret name is specified use it otherwise don't set anything
	if len(s.backup.Spec.BackupSecretName) != 0 {
		container.EnvFrom = []core.EnvFromSource{
			{
				SecretRef: &core.SecretEnvSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.backup.Spec.BackupSecretName,
					},
				},
			},
		}
	}
	return []core.Container{container}
}

func (s *deletionJobSyncer) recordWEventOnCluster(reason, msg string) {
	if s.cluster != nil {
		s.recorder.Eventf(s.cluster, "Warning", reason, msg)
	}
}

func bucketForRclone(name string) string {
	return strings.Replace(name, "://", ":", 1)
}

func getJobStatus(job *batch.Job) (bool, bool) {
	completed := false
	if completCond := jobCondition(batch.JobComplete, job); completCond != nil {
		if completCond.Status == core.ConditionTrue {
			completed = true
		}
	}

	failed := false
	if failCond := jobCondition(batch.JobFailed, job); failCond != nil {
		if failCond.Status == core.ConditionTrue {
			failed = true
		}
	}

	return completed, failed
}

func addFinalizer(in *api.MysqlBackup, finalizer string) {
	for _, f := range in.Finalizers {
		if f == finalizer {
			// finalizer already exists
			return
		}
	}

	// initialize list
	if len(in.Finalizers) == 0 {
		in.Finalizers = []string{}
	}

	in.Finalizers = append(in.Finalizers, finalizer)
}

func removeFinalizer(in *api.MysqlBackup, finalizer string) {
	var (
		index int
		f     string
	)
	for index, f = range in.Finalizers {
		if f == finalizer {
			in.Finalizers = append(in.Finalizers[:index], in.Finalizers[index+1:]...)
		}
	}

}
