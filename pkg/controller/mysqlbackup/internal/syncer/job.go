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

	"github.com/presslabs/controller-util/syncer"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	backupwrap "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlbackup"
	clusterwrap "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var log = logf.Log.WithName("mysqlbackup.syncer.job")

type jobSyncer struct {
	backup  *backupwrap.Wrapper
	cluster *api.MysqlCluster

	job *batch.Job
	opt *options.Options
}

// NewJobSyncer returns a syncer for backup jobs
func NewJobSyncer(backup *api.MysqlBackup, cluster *api.MysqlCluster, opt *options.Options) syncer.Interface {
	wBackup := backupwrap.New(backup)
	obj := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wBackup.GetNameForJob(),
			Namespace: backup.Namespace,
		},
	}

	return &jobSyncer{
		backup:  wBackup,
		cluster: cluster,
		job:     obj,
		opt:     opt,
	}
}

func (s *jobSyncer) GetObject() runtime.Object { return s.job }
func (s *jobSyncer) GetOwner() runtime.Object  { return s.backup.MysqlBackup }
func (s *jobSyncer) GetEventReasonForError(err error) syncer.EventReason {
	return syncer.BasicEventReason("Job", err)
}

func (s *jobSyncer) SyncFn(in runtime.Object) error {
	out := in.(*batch.Job)

	if len(s.backup.GetBackupURL(s.cluster)) == 0 {
		log.Info("can't get bucketURI", "cluster", s.cluster, "backup", s.backup)
		return fmt.Errorf("can't get bucketURI")
	}

	// check if job is already created an just update the status
	if !out.ObjectMeta.CreationTimestamp.IsZero() {
		s.updateStatus(out)
		return nil
	}

	out.Labels = map[string]string{
		"cluster": s.backup.Spec.ClusterName,
	}

	out.Spec.Template.Spec = s.ensurePodSpec(out.Spec.Template.Spec)
	return nil
}

func (s *jobSyncer) getBackupSecretName() string {
	if len(s.backup.Spec.BackupSecretName) > 0 {
		return s.backup.Spec.BackupSecretName
	}

	return s.cluster.Spec.BackupSecretName
}

// getBackupCandidate returns the hostname of the first not-lagged and
// replicating slave node, else returns the master node.
func (s *jobSyncer) getBackupCandidate() string {
	wCluster := clusterwrap.NewMysqlClusterWrapper(s.cluster)
	for _, node := range s.cluster.Status.Nodes {
		master := wCluster.GetNodeCondition(node.Name, api.NodeConditionMaster)
		replicating := wCluster.GetNodeCondition(node.Name, api.NodeConditionReplicating)
		lagged := wCluster.GetNodeCondition(node.Name, api.NodeConditionLagged)

		isMaster := master.Status == core.ConditionTrue
		isReplicating := replicating != nil && replicating.Status == core.ConditionTrue
		isLagged := lagged != nil && lagged.Status == core.ConditionTrue

		if master == nil || replicating == nil || lagged == nil {
			continue
		}

		// select a node that is not master is replicating and is not lagged
		if !isMaster && isReplicating && !isLagged {
			return node.Name
		}
	}
	log.Info("no healthy slave node found so returns the master node", "default_node", wCluster.GetPodHostname(0),
		"cluster", s.cluster)
	return wCluster.GetPodHostname(0)
}

func (s *jobSyncer) ensurePodSpec(in core.PodSpec) core.PodSpec {
	if len(in.Containers) == 0 {
		in.Containers = make([]core.Container, 1)
	}

	in.RestartPolicy = core.RestartPolicyNever

	in.Containers[0].Name = "backup"
	in.Containers[0].Image = s.opt.HelperImage
	in.Containers[0].ImagePullPolicy = core.PullIfNotPresent
	in.Containers[0].Args = []string{
		"take-backup-to",
		s.getBackupCandidate(),
		s.backup.GetBackupURL(s.cluster),
	}

	boolTrue := true
	in.Containers[0].Env = []core.EnvVar{
		core.EnvVar{
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
		},
		core.EnvVar{
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
		},
	}

	if len(s.getBackupSecretName()) != 0 {
		in.Containers[0].EnvFrom = []core.EnvFromSource{
			core.EnvFromSource{
				SecretRef: &core.SecretEnvSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.getBackupSecretName(),
					},
				},
			},
		}
	}
	return in
}

func (s *jobSyncer) updateStatus(job *batch.Job) {
	// check for completion condition
	if cond := jobCondition(batch.JobComplete, job); cond != nil {
		s.backup.UpdateStatusCondition(api.BackupComplete, cond.Status, cond.Reason, cond.Message)

		if cond.Status == core.ConditionTrue {
			s.backup.Status.Completed = true
		}
	}

	// check for failed condition
	if cond := jobCondition(batch.JobFailed, job); cond != nil {
		s.backup.UpdateStatusCondition(api.BackupFailed, cond.Status, cond.Reason, cond.Message)
	}
}

func jobCondition(condType batch.JobConditionType, job *batch.Job) *batch.JobCondition {
	for _, c := range job.Status.Conditions {
		if c.Type == condType {
			return &c
		}
	}

	return nil
}
