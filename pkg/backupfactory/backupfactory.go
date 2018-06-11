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

package backupfactory

import (
	"context"
	"fmt"
	"strings"
	"time"

	kbatch "github.com/appscode/kutil/batch/v1"
	"github.com/golang/glog"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	clientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	"github.com/presslabs/mysql-operator/pkg/util"
)

type Interface interface {
	SetDefaults() error
	Sync(ctx context.Context) error
}

type bFactory struct {
	backup  *api.MysqlBackup
	cluster *api.MysqlCluster

	k8Client kubernetes.Interface
	myClient clientset.Interface
}

func New(backup *api.MysqlBackup, k8client kubernetes.Interface,
	myClient clientset.Interface, cluster *api.MysqlCluster) Interface {
	return &bFactory{
		backup:   backup,
		cluster:  cluster,
		k8Client: k8client,
		myClient: myClient,
	}
}

func (f *bFactory) Sync(ctx context.Context) error {
	meta := metav1.ObjectMeta{
		Name:      f.getJobName(),
		Namespace: f.backup.Namespace,
		Labels: map[string]string{
			"cluster": f.backup.Spec.ClusterName,
		},
		OwnerReferences: []metav1.OwnerReference{
			f.backup.AsOwnerReference(),
		},
	}

	if len(f.GetBackupUri()) == 0 {
		glog.Errorf("The backupUri for backupt: '%s' and cluster: '%s' not specified,"+
			" neither in backup or cluster!", f.backup.Name, f.cluster.Name)
		return fmt.Errorf("backupUri not specified")
	}

	_, _, err := kbatch.CreateOrPatchJob(f.k8Client, meta, func(in *batch.Job) *batch.Job {
		if len(in.Spec.Template.Spec.Containers) == 0 {
			in.Spec.Template.Spec = f.ensurePodSpec(in.Spec.Template.Spec)
		} else {
			f.updateStatus(in)
		}
		return in
	})

	return err
}

func (f *bFactory) getJobName() string {
	return fmt.Sprintf("%s-backupjob", f.backup.Name)
}

func (f *bFactory) ensurePodSpec(in core.PodSpec) core.PodSpec {
	if len(in.Containers) == 0 {
		in.Containers = make([]core.Container, 1)
	}

	in.RestartPolicy = core.RestartPolicyNever

	in.Containers[0].Name = "backup"
	in.Containers[0].Image = f.backup.GetHelperImage()
	in.Containers[0].ImagePullPolicy = core.PullIfNotPresent
	in.Containers[0].Args = []string{
		"take-backup-to",
		f.cluster.GetBackupCandidate(),
		f.GetBackupUri(),
	}

	if len(f.GetBackupSecretName()) != 0 {
		in.Containers[0].EnvFrom = []core.EnvFromSource{
			core.EnvFromSource{
				SecretRef: &core.SecretEnvSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.GetBackupSecretName(),
					},
				},
			},
		}
	}
	return in
}

func (f *bFactory) SetDefaults() error {
	if completeCond := f.backup.GetCondition(api.BackupComplete); completeCond != nil {
		// initialization was done. Skip
		glog.V(3).Info("Backup object is initialized, skip initialization.")
		return nil
	}

	f.backup.UpdateStatusCondition(api.BackupComplete, core.ConditionUnknown, "set defaults",
		"First initialization of backup")

	// mark backup as not in final state
	f.backup.Status.Completed = false

	// update status backup uri
	f.backup.Status.BackupUri = f.GetBackupUri()

	return nil
}

func (f *bFactory) GetBackupUri() string {
	if len(f.backup.Status.BackupUri) > 0 {
		return f.backup.Status.BackupUri
	}

	if len(f.backup.Spec.BackupUri) > 0 {
		return f.backup.Spec.BackupUri
	}

	if len(f.cluster.Spec.BackupUri) > 0 {
		return getBucketUri(f.cluster.Name, f.cluster.Spec.BackupUri)
	}

	return ""
}

func getBucketUri(cluster, bucket string) string {
	if strings.HasSuffix(bucket, "/") {
		bucket = bucket[:len(bucket)-1]
	}
	t := time.Now()
	return bucket + fmt.Sprintf(
		"/%s-%s.xbackup.gz", cluster, t.Format("2006-01-02T15:04:05"),
	)
}

func (f *bFactory) GetBackupSecretName() string {
	if len(f.backup.Spec.BackupSecretName) > 0 {
		return f.backup.Spec.BackupSecretName
	}

	return f.cluster.Spec.BackupSecretName
}

func (f *bFactory) updateStatus(job *batch.Job) {
	glog.V(2).Infof("Updating status of '%s' backup", f.backup.Name)

	if i, exists := util.JobConditionIndex(batch.JobComplete, job.Status.Conditions); exists {
		cond := job.Status.Conditions[i]
		f.backup.UpdateStatusCondition(api.BackupComplete, cond.Status,
			cond.Reason, cond.Message)

		if cond.Status == core.ConditionTrue {
			f.backup.Status.Completed = true
		}
	}

	if i, exists := util.JobConditionIndex(batch.JobFailed, job.Status.Conditions); exists {
		cond := job.Status.Conditions[i]
		f.backup.UpdateStatusCondition(api.BackupFailed, cond.Status, cond.Reason, cond.Message)
	}
}
