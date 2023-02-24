/*
Copyright 2020 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://wwb.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package mysqlbackup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/klog/v2/klogr"

	logf "github.com/presslabs/controller-util/pkg/log"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func TestMySQLBackupWrapper(t *testing.T) {
	klog.SetOutput(GinkgoWriter)
	logf.SetLogger(klogr.New())

	RegisterFailHandler(Fail)
	RunSpecs(t, "MySQLBackup wrapper unit tests")
}

var _ = Describe("MySQL backup unit tests", func() {
	It("should generate the correct cleanup job backup", func() {
		backup := New(&api.MysqlBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-name",
			},
		})

		Expect(backup.GetNameForDeletionJob()).To(Equal("backup-name-cleanup"))

		backup.Name = "super-long-backup-name-for-testing-cleanup-job-name-generator"
		Expect(backup.GetNameForDeletionJob()).To(Equal("super-long-backup-name-for-testing-cleanup-j-2387987902-cleanup"))
		Expect(len(backup.GetNameForDeletionJob())).To(BeNumerically("<=", 63))

		backup.Name = "not-too-long-backup-name-for-testing-cleanup-job-test"
		Expect(backup.GetNameForDeletionJob()).To(Equal("not-too-long-backup-name-for-testing-cleanup-job-test-cleanup"))
		Expect(len(backup.GetNameForDeletionJob())).To(BeNumerically("<=", 63))
	})

	It("should generate the correct backup job name", func() {
		backup := New(&api.MysqlBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-name",
			},
		})

		Expect(backup.GetNameForJob()).To(Equal("backup-name-backup"))

		backup.Name = "super-long-backup-name-for-testing-backup-job-name-generator"
		Expect(backup.GetNameForJob()).To(Equal("super-long-backup-name-for-testing-backup-jo-4133418200-backup"))
		Expect(len(backup.GetNameForJob())).To(BeNumerically("<=", 63))

		backup.Name = "not-too-long-backup-name-for-testing-backup-job-test"
		Expect(backup.GetNameForJob()).To(Equal("not-too-long-backup-name-for-testing-backup-job-test-backup"))
		Expect(len(backup.GetNameForJob())).To(BeNumerically("<=", 63))
	})
})
