/*
Copyright 2020 Pressinfra SRL

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
package v1alpha1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MysqlDatabase CRUD", func() {
	var created *MysqlDatabase
	var key types.NamespacedName

	BeforeEach(func() {
		key = types.NamespacedName{Name: "foo", Namespace: "default"}
		created = &MysqlDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: MysqlDatabaseSpec{
				Database: "foo",
			},
		}
	})

	AfterEach(func() {
		c.Delete(context.TODO(), created)
	})

	Context("for a valid config", func() {
		It("should provide CRUD access to the object", func() {
			// Test Create
			fetched := &MysqlDatabase{}
			Expect(c.Create(context.TODO(), created)).NotTo(HaveOccurred())

			Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
			Expect(fetched).To(Equal(created))

			// Test Updating the Labels
			updated := fetched.DeepCopy()
			updated.Labels = map[string]string{"hello": "world"}
			Expect(c.Update(context.TODO(), updated)).NotTo(HaveOccurred())

			Expect(c.Get(context.TODO(), key, fetched)).NotTo(HaveOccurred())
			Expect(fetched).To(Equal(updated))

			// Test Delete
			Expect(c.Delete(context.TODO(), fetched)).NotTo(HaveOccurred())
			Expect(c.Get(context.TODO(), key, fetched)).To(HaveOccurred())

		})
	})

})
