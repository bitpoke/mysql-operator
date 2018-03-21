package mysqlcluster

import (
	kbatch "github.com/appscode/kutil/batch/v1beta1"
	batch "k8s.io/api/batch/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
)

func (f *cFactory) syncBackupCronJob() (state string, err error) {
	meta := metav1.ObjectMeta{
		Name:            f.cl.GetNameForResource(api.BackupCronJob),
		Labels:          f.getLabels(map[string]string{}),
		OwnerReferences: f.getOwnerReferences(),
		Namespace:       f.namespace,
	}

	_, act, err := kbatch.CreateOrPatchCronJob(f.client, meta,
		func(in *batch.CronJob) *batch.CronJob {
			in.Spec.Schedule = f.cl.Spec.BackupSchedule
			in.Spec.ConcurrencyPolicy = batch.ForbidConcurrent
			in.Spec.JobTemplate.Spec.Template.Spec = f.ensurePodTemplate(
				in.Spec.JobTemplate.Spec.Template.Spec)

			return in
		})

	state = getStatusFromKVerb(act)
	return
}

func (f *cFactory) ensurePodTemplate(in core.PodSpec) core.PodSpec {
	if len(in.Containers) == 0 {
		in.Containers = make([]core.Container, 1)
	}
	in.Containers[0].Name = "schedule-backup"
	in.Containers[0].Image = f.cl.Spec.GetTitaniumImage()
	in.Containers[0].ImagePullPolicy = core.PullIfNotPresent
	in.Containers[0].Args = []string{
		"schedule-backup",
		f.cl.Name,
	}

	return in
}
