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
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	syncerpkg "github.com/presslabs/controller-util/syncer"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlbackup"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var _ = Describe("MysqlBackup remove job syncer", func() {
	var (
		cluster  *mysqlcluster.MysqlCluster
		backup   *mysqlbackup.MysqlBackup
		syncer   *deletionJobSyncer
		recorder *record.FakeRecorder
	)

	BeforeEach(func() {
		clusterName := fmt.Sprintf("cluster-%d", rand.Int31())
		name := fmt.Sprintf("backup-%d", rand.Int31())
		recorder = record.NewFakeRecorder(100)
		ns := "default"

		two := int32(2)
		cluster = mysqlcluster.New(&api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
			Spec: api.MysqlClusterSpec{
				Replicas:   &two,
				SecretName: "a-secret",
			},
		})

		backup = mysqlbackup.New(&api.MysqlBackup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: api.MysqlBackupSpec{
				ClusterName: cluster.Name,
				BackupURL:   "gs://bucket/",
			},
		})

		syncer = &deletionJobSyncer{
			cluster:  cluster,
			backup:   backup,
			opt:      options.GetOptions(),
			recorder: recorder,
			schema:   scheme.Scheme,
		}
	})

	It("should skip job creation when no needed", func() {
		delJob := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{Name: backup.Name + "-job", Namespace: backup.Namespace},
		}
		backup.Spec.RemoteDeletePolicy = api.Retain
		// skip job creation because backup is set to soft delete
		Expect(syncer.SyncFn(delJob)).To(Equal(syncerpkg.ErrIgnore))

		backup.Spec.RemoteDeletePolicy = api.Delete
		// skip job creation because backup is not deleted
		Expect(syncer.SyncFn(delJob)).To(Equal(syncerpkg.ErrIgnore))
		Expect(backup.Finalizers).To(ContainElement(RemoteStorageFinalizer))

		deletionTime := metav1.NewTime(time.Now())
		backup.DeletionTimestamp = &deletionTime
		backup.UpdateStatusCondition(api.BackupFailed, core.ConditionTrue, "", "")
		// skip job creation because backup is not finished
		Expect(syncer.SyncFn(delJob)).To(Equal(syncerpkg.ErrIgnore))
		Expect(backup.Finalizers).ToNot(ContainElement(RemoteStorageFinalizer))
	})

	It("should create the job and update backup finalizer", func() {
		delJob := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{Name: backup.Name + "-job", Namespace: backup.Namespace},
		}
		backup.Spec.RemoteDeletePolicy = api.Delete
		deletionTime := metav1.NewTime(time.Now())
		backup.DeletionTimestamp = &deletionTime
		Expect(syncer.SyncFn(delJob)).To(Succeed())

		// check that the job initialized
		Expect(delJob.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(backup.Finalizers).To(ContainElement(RemoteStorageFinalizer))

		delJob.Status.Conditions = []batch.JobCondition{
			batch.JobCondition{
				Type:   batch.JobComplete,
				Status: core.ConditionTrue,
			},
			batch.JobCondition{
				Type:   batch.JobFailed,
				Status: core.ConditionTrue,
			},
		}
		Expect(syncer.SyncFn(delJob)).To(Succeed())
		Expect(backup.Finalizers).ToNot(ContainElement(RemoteStorageFinalizer))
		Expect(recorder.Events).To(Receive(ContainSubstring(RemoteDeletionFailedEvent)))

		delJob.Status.Conditions = []batch.JobCondition{
			batch.JobCondition{
				Type:   batch.JobComplete,
				Status: core.ConditionFalse,
			},
		}
		Expect(syncer.SyncFn(delJob)).To(Succeed())
		Expect(backup.Finalizers).To(ContainElement(RemoteStorageFinalizer))

		delJob.Status.Conditions = []batch.JobCondition{
			batch.JobCondition{
				Type:   batch.JobComplete,
				Status: core.ConditionTrue,
			},
			batch.JobCondition{
				Type:   batch.JobFailed,
				Status: core.ConditionFalse,
			},
		}

		Expect(syncer.SyncFn(delJob)).To(Succeed())
		Expect(backup.Finalizers).ToNot(ContainElement(RemoteStorageFinalizer))
	})
})
