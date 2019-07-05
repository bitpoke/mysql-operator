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
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var log = logf.Log.WithName("mysqlbackup.syncer.job")

type jobSyncer struct {
	backup  *mysqlbackup.MysqlBackup
	cluster *mysqlcluster.MysqlCluster

	opt *options.Options
}

// NewJobSyncer returns a syncer for backup jobs
func NewJobSyncer(c client.Client, s *runtime.Scheme, backup *mysqlbackup.MysqlBackup, cluster *mysqlcluster.MysqlCluster, opt *options.Options) syncer.Interface {
	obj := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.GetNameForJob(),
			Namespace: backup.Namespace,
		},
	}

	sync := &jobSyncer{
		backup:  backup,
		cluster: cluster,
		opt:     opt,
	}

	return syncer.NewObjectSyncer("Job", backup.Unwrap(), obj, c, s, sync.SyncFn)
}

func (s *jobSyncer) SyncFn(in runtime.Object) error {
	out := in.(*batch.Job)

	if s.backup.Status.Completed {
		log.V(1).Info("backup already completed", "name", s.backup.Name)
		// skip doing anything
		return syncer.ErrIgnore
	}

	if len(s.backup.GetBackupURL(s.cluster)) == 0 {
		log.Info("can't get backupURL", "cluster", s.cluster, "backup", s.backup)
		return fmt.Errorf("can't get backupURL")
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

// getBackupCandidate returns the hostname of the first not-lagged and
// replicating slave node, else returns the master node.
func (s *jobSyncer) getBackupCandidate() string {
	for _, node := range s.cluster.Status.Nodes {
		master := s.cluster.GetNodeCondition(node.Name, api.NodeConditionMaster)
		replicating := s.cluster.GetNodeCondition(node.Name, api.NodeConditionReplicating)
		lagged := s.cluster.GetNodeCondition(node.Name, api.NodeConditionLagged)

		if master == nil || replicating == nil || lagged == nil {
			continue
		}

		isMaster := master.Status == core.ConditionTrue
		isReplicating := replicating.Status == core.ConditionTrue
		isLagged := lagged.Status == core.ConditionTrue

		// select a node that is not master is replicating and is not lagged
		if !isMaster && isReplicating && !isLagged {
			return node.Name
		}
	}
	log.Info("no healthy slave node found so returns the master node", "default_node", s.cluster.GetPodHostname(0),
		"cluster", s.cluster)
	return s.cluster.GetPodHostname(0)
}

func (s *jobSyncer) ensurePodSpec(in core.PodSpec) core.PodSpec {
	if len(in.Containers) == 0 {
		in.Containers = make([]core.Container, 1)
	}

	in.RestartPolicy = core.RestartPolicyNever
	in.ImagePullSecrets = []core.LocalObjectReference{
		{Name: s.opt.ImagePullSecretName},
	}

	in.Containers[0].Name = "backup"
	in.Containers[0].Image = s.opt.SidecarImage
	in.Containers[0].ImagePullPolicy = s.opt.ImagePullPolicy
	in.Containers[0].Args = []string{
		"take-backup-to",
		s.getBackupCandidate(),
		s.backup.GetBackupURL(s.cluster),
	}

	in.ServiceAccountName = s.cluster.Spec.PodSpec.ServiceAccountName

	in.Affinity = s.cluster.Spec.PodSpec.Affinity
	in.ImagePullSecrets = s.cluster.Spec.PodSpec.ImagePullSecrets
	in.NodeSelector = s.cluster.Spec.PodSpec.NodeSelector
	in.PriorityClassName = s.cluster.Spec.PodSpec.PriorityClassName
	in.Tolerations = s.cluster.Spec.PodSpec.Tolerations

	boolTrue := true
	in.Containers[0].Env = []core.EnvVar{
		{
			Name: "BACKUP_USER",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.cluster.GetNameForResource(mysqlcluster.Secret),
					},
					Key:      "BACKUP_USER",
					Optional: &boolTrue,
				},
			},
		},
		{
			Name: "BACKUP_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.cluster.GetNameForResource(mysqlcluster.Secret),
					},
					Key:      "BACKUP_PASSWORD",
					Optional: &boolTrue,
				},
			},
		},
	}

	if len(s.backup.Spec.BackupSecretName) != 0 {
		in.Containers[0].EnvFrom = []core.EnvFromSource{
			core.EnvFromSource{
				SecretRef: &core.SecretEnvSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: s.backup.Spec.BackupSecretName,
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

		if cond.Status == core.ConditionTrue {
			s.backup.Status.Completed = true
		}
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
