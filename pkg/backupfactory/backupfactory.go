package backupfactory

import (
	"context"
	"fmt"

	kbatch "github.com/appscode/kutil/batch/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	api "github.com/presslabs/titanium/pkg/apis/titanium/v1alpha1"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
)

type Interface interface {
	Sync(ctx context.Context) error
}

type bFactory struct {
	backup  *api.MysqlBackup
	cluster *api.MysqlCluster

	k8Client kubernetes.Interface
	tClient  clientset.Interface
}

func New(backup *api.MysqlBackup, k8client kubernetes.Interface,
	tclient clientset.Interface, cluster *api.MysqlCluster) Interface {
	return &bFactory{
		backup:   backup,
		cluster:  cluster,
		k8Client: k8client,
		tClient:  tclient,
	}
}

func (f *bFactory) Sync(ctx context.Context) error {
	meta := metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s-backup", f.backup.Name, f.backup.ClusterName),
		Namespace: f.backup.Namespace,
		Labels: map[string]string{
			"cluster": f.backup.ClusterName,
		},
		OwnerReferences: []metav1.OwnerReference{
			f.backup.AsOwnerReference(),
		},
	}
	_, _, err := kbatch.CreateOrPatchJob(f.k8Client, meta, func(in *batch.Job) *batch.Job {
		in.Spec.Template.Spec = f.ensurePodSpec(in.Spec.Template.Spec)
		return in
	})

	return err
}

func (f *bFactory) ensurePodSpec(in core.PodSpec) core.PodSpec {
	if len(in.Containers) == 0 {
		in.Containers = make([]core.Container, 1)
	}

	in.RestartPolicy = core.RestartPolicyNever

	in.Containers[0].Name = "backup"
	in.Containers[0].Image = f.backup.GetTitaniumImage()
	in.Containers[0].ImagePullPolicy = core.PullIfNotPresent
	in.Containers[0].Args = []string{
		"take-backup-to",
		f.cluster.GetLastSlaveHost(),
		f.backup.Status.BucketUri,
	}

	if len(f.backup.Spec.BucketSecretName) != 0 {
		in.Containers[0].EnvFrom = []core.EnvFromSource{
			core.EnvFromSource{
				SecretRef: &core.SecretEnvSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: f.backup.Spec.BucketSecretName,
					},
				},
			},
		}
	}
	return in
}
