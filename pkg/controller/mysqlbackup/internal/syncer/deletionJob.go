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
	"github.com/imdario/mergo"
	"github.com/presslabs/controller-util/mergo/transformers"
	"github.com/presslabs/controller-util/syncer"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	// RemoteStorageFinalizer is the finalizer name used when hardDelete policy is used
	RemoteStorageFinalizer = "backups.mysql.presslabs.org/remote-storage"
)

type deletionJobSyncer struct {
	backup *mysqlbackup.MysqlBackup
	opt    *options.Options
}

// NewRemoteJobSyncer returns a job syncer for hard deletion job. The job which removes the backup
// from remote storage.
func NewRemoteJobSyncer(c client.Client, s *runtime.Scheme,
	backup *mysqlbackup.MysqlBackup, opt *options.Options) syncer.Interface {

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.GetNameForDeletionJob(),
			Namespace: backup.Namespace,
		},
	}

	jobSyncer := deletionJobSyncer{
		backup: backup,
		opt:    opt,
	}

	return syncer.NewObjectSyncer("Backup", backup.Unwrap(), job, c, s, jobSyncer.SyncFn)
}

func (s *deletionJobSyncer) SyncFn(in runtime.Object) error {
	out := in.(*batch.Job)

	if s.backup.Spec.DeletePolicy == api.SoftDelete {
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

	// check if the job is created and if not create it
	if out.ObjectMeta.CreationTimestamp.IsZero() {
		out.Labels = map[string]string{
			"backup":      s.backup.Name,
			"cleanup-job": "true",
		}

		err := mergo.Merge(&out.Spec.Template.Spec, s.ensurePodSpec(),
			mergo.WithTransformers(transformers.PodSpec))
		if err != nil {
			return err
		}
	}

	completed, failed := getJobStatus(out)
	if completed && !failed {
		removeFinalizer(s.backup.Unwrap(), RemoteStorageFinalizer)
	}

	return nil
}

func (s *deletionJobSyncer) ensurePodSpec() core.PodSpec {
	return core.PodSpec{
		Containers: s.ensureContainers(),
		ImagePullSecrets: []core.LocalObjectReference{
			{Name: s.opt.ImagePullSecretName},
		},
	}
}

func (s *deletionJobSyncer) ensureContainers() []core.Container {
	return []core.Container{
		{
			Name:            "delete",
			Image:           s.opt.SidecarImage,
			ImagePullPolicy: s.opt.ImagePullPolicy,
			Args: []string{
				"rclone", "--config=/etc/rclone.conf", "delete",
				s.backup.Status.BackupURI,
			},
		},
	}
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
			break
		}
	}

	in.Finalizers = append(in.Finalizers[:index], in.Finalizers[index+1:]...)
}
