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

	kbatch "github.com/appscode/kutil/batch/v1beta1"
	"github.com/golang/glog"
	batch "k8s.io/api/batch/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func (f *cFactory) syncBackupCronJob() (state string, err error) {
	if len(f.cluster.Spec.BackupSchedule) == 0 {
		glog.Infof("[syncBackupCronJob]: no schedule specified for cluster: %s", f.cluster.Name)
		state = statusSkip
		return
	}

	meta := metav1.ObjectMeta{
		Name:            f.cluster.GetNameForResource(api.BackupCronJob),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kbatch.CreateOrPatchCronJob(f.client, meta,
		func(in *batch.CronJob) *batch.CronJob {
			backoffLimit := int32(3)

			in.Spec.Schedule = f.cluster.Spec.BackupSchedule
			in.Spec.ConcurrencyPolicy = batch.ForbidConcurrent
			in.Spec.JobTemplate.Spec.BackoffLimit = &backoffLimit
			in.Spec.JobTemplate.Spec.Template.Spec = f.ensurePodTemplate(
				in.Spec.JobTemplate.Spec.Template.Spec)

			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) ensurePodTemplate(spec core.PodSpec) core.PodSpec {
	if len(spec.Containers) == 0 {
		spec.Containers = make([]core.Container, 1)
	}

	spec.RestartPolicy = core.RestartPolicyOnFailure
	spec.ServiceAccountName = f.opt.BackupSchedulerServiceAccountName

	spec.Containers[0].Name = "schedule-backup"
	spec.Containers[0].Image = f.cluster.Spec.GetHelperImage()
	spec.Containers[0].ImagePullPolicy = core.PullIfNotPresent
	spec.Containers[0].Args = []string{
		"schedule-backup",
		fmt.Sprintf("--namespace=%s", f.cluster.Namespace),
		f.cluster.Name,
	}

	return spec
}
