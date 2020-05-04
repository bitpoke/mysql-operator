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

// NOTE: using of _test package name because cyclic import of package mysql (required by mysql/fake)
package mysql_test

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	. "github.com/presslabs/mysql-operator/pkg/internal/mysql"
	"github.com/presslabs/mysql-operator/pkg/internal/mysql/fake"
)

func TestMySQLInterface(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MySQL Interface tests")
}

var _ = Describe("MySQL User Interface tests", func() {
	var (
		sql *fake.SQLRunner
	)

	BeforeEach(func() {
		sql = fake.NewQueryRunner(false)
	})

	Context("user related", func() {
		var (
			user            string
			pwd             string
			allowedHosts    []string
			permissions     []mysqlv1alpha1.MysqlPermission
			resourceOptions corev1.ResourceList
		)

		BeforeEach(func() {
			user = "mysqlusername"
			pwd = "random-password"
			allowedHosts = []string{"localhost"}
			permissions = []mysqlv1alpha1.MysqlPermission{
				{
					Schema:      "test_db",
					Tables:      []string{"*"},
					Permissions: []string{"PERM1", "PERM2"},
				},
			}
			resourceOptions = corev1.ResourceList{
				mysqlv1alpha1.AccountResourceMaxUserConnections: resource.MustParse("10"),
			}
		})

		It("should build the right queries for user creation", func() {
			assertQuery(sql,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ? WITH ?=?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHosts[0], pwd, user, allowedHosts[0], pwd, "MAX_USER_CONNECTIONS", 10, user, allowedHosts[0],
			)
			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, allowedHosts, permissions, resourceOptions)).To(Succeed())
		})

		It("should build the right queries for user creation", func() {
			allowedHosts = []string{"test1", "test2"}
			assertQuery(sql,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ? WITH ?=?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?, ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHosts[0], pwd,
				user, allowedHosts[1], pwd,
				user, allowedHosts[0], pwd,
				user, allowedHosts[1], pwd,
				"MAX_USER_CONNECTIONS", 10,
				user, allowedHosts[0],
				user, allowedHosts[1],
			)
			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, allowedHosts, permissions, resourceOptions)).To(Succeed())
		})

		It("should build queries with no resource limits", func() {
			assertQuery(sql,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHosts[0], pwd, user, allowedHosts[0], pwd, user, allowedHosts[0],
			)

			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, allowedHosts, permissions, nil)).To(Succeed())
		})

		It("should build queries with more resource limits", func() {
			resourceOptions["MAX_QUERIES_PER_HOUR"] = resource.MustParse("100")
			assertQuery(sql,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ? WITH ?=? ?=?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHosts[0], pwd, user, allowedHosts[0], pwd, "MAX_USER_CONNECTIONS", 10, "MAX_QUERIES_PER_HOUR", 100, user, allowedHosts[0],
			)

			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, allowedHosts, permissions, resourceOptions)).To(Succeed())
		})
		It("should not accept a user without allowedHosts", func() {
			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, []string{}, []mysqlv1alpha1.MysqlPermission{}, corev1.ResourceList{})).ToNot(Succeed())
		})

		It("should create the user without permissions", func() {
			assertQuery(sql,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHosts[0], pwd, user, allowedHosts[0], pwd,
			)
			Expect(CreateUserIfNotExists(context.TODO(), sql, user, pwd, allowedHosts, []mysqlv1alpha1.MysqlPermission{}, corev1.ResourceList{})).To(Succeed())
		})
	})

})

func assertQuery(qr *fake.SQLRunner, expectedQuery string, expectedArgs ...interface{}) {
	expectedQueryRunnerCall := func(query string, args ...interface{}) error {
		defer GinkgoRecover()

		Expect(query).To(Equal(expectedQuery))
		Expect(args).To(ConsistOf(expectedArgs...))

		return nil
	}

	qr.AddExpectedCalls(expectedQueryRunnerCall)
}
