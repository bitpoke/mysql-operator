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

// nolint: errcheck
package mysqlcluster

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

var _ = Describe("ConfigMap syncer", func() {
	var cluster *api.MysqlCluster

	BeforeEach(func() {
		cluster = &api.MysqlCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: api.MysqlClusterSpec{
				Replicas:   1,
				SecretName: "the-secret",
				MysqlConf:  api.MysqlConf{},
			},
		}
	})

	AfterEach(func() {
		c.Delete(context.TODO(), cluster)
	})

	Describe("tests", func() {
		Context("for a valid config", func() {
			It("should be created and updated", func() {
				syncer := NewConfigMapSyncer(cluster)
				cm := syncer.GetExistingObjectPlaceholder().(*core.ConfigMap)

				Expect(cm.Name).To(Equal(cluster.GetNameForResource(api.ConfigMap)))
				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())

				Expect(cm.ObjectMeta.Annotations).Should(HaveKey("config_hash"))
				oldHash := cm.ObjectMeta.Annotations["config_hash"]

				// update cluster config should reflect in
				cluster.Spec.MysqlConf["ceva_nou"] = "1"
				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())
				Expect(cm.ObjectMeta.Annotations["config_hash"]).ToNot(Equal(oldHash))
			})
		})

		Context("update cluster MysqlConfig", func() {
			It("should reflect in config hash", func() {
				syncer := NewConfigMapSyncer(cluster)
				cm := syncer.GetExistingObjectPlaceholder().(*core.ConfigMap)
				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())

				oldHash := cm.ObjectMeta.Annotations["config_hash"]
				cluster.Spec.MysqlConf["ceva_nou"] = "1"

				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())
				Expect(cm.ObjectMeta.Annotations["config_hash"]).ToNot(Equal(oldHash))
			})

			It("should not change multiple times", func() {
				syncer := NewConfigMapSyncer(cluster)
				cm := syncer.GetExistingObjectPlaceholder().(*core.ConfigMap)
				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())

				cluster.Spec.MysqlConf["ceva_nou"] = "1"

				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())
				oldHash := cm.ObjectMeta.Annotations["config_hash"]

				Expect(syncer.Sync(cm)).NotTo(HaveOccurred())
				Expect(cm.ObjectMeta.Annotations["config_hash"]).To(Equal(oldHash))
			})
		})
	})
})
