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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MysqlCluster CRUD", func() {
	var cluster *api.MysqlCluster

	BeforeEach(func() {
		cluster = &api.MysqlCluster{
			Name:      "foo",
			Namespace: "default",
			Spec: api.MysqlClusterSpec{
				Replicas:   1,
				SecretName: "the-secret",
			},
		}

	})

	AfterEach(func() {
		c.Delete(context.TODO(), cluster)
	})

	Context("for a valid config", func() {
		It("should be created and updated", func() {
			syncer := NewConfigMapSyncer(cluster)
			cm := syncer.GetExistingObjectPlaceholder()

			Expect(cm.Name).To(Equal(cluster.GetNameForResource(api.ConfigMap)))
			Expect(syncer.Sync(cm)).NotTo(HaveOcurred())

			Expect(cm.ObjectMeta.Annotations).Should(ContainElement("config_hash"))

			// update cluster config should reflect in
			cluster.Spec.MysqlConf["ceva_nou"] = "1"
			Expect(syncer.Sync(cm)).NotTo(HaveOcurred())
			Expect(cm.ObjectMeta.Annotations["config_hash"]).ToNot(Equal(old_hash))
		})
	})

	Context("update cluster MysqlConfig", func() {
		syncer := NewConfigMapSyncer(cluster)
		cm := syncer.GetExistingObjectPlaceholder()
		Expect(syncer.Sync(cm)).NotTo(HaveOcurred())

		old_hash := cm.ObjectMeta.Annotations["config_hash"]
		cluster.Spec.MysqlConf["ceva_nou"] = "1"

		It("should reflect in config hash", func() {
			Expect(syncer.Sync(cm)).NotTo(HaveOcurred())
			Expect(cm.ObjectMeta.Annotations["config_hash"]).ToNot(Equal(old_hash))
		})

		It("should not change multiple times", func() {
			Expect(syncer.Sync(cm)).NotTo(HaveOcurred())
			old_hash = cm.ObjectMeta.Annotations["config_hash"]

			Expect(syncer.Sync(cm)).NotTo(HaveOcurred())
			Expect(cm.ObjectMeta.Annotations["config_hash"]).To(Equal(old_hash))
		})
	})
})
