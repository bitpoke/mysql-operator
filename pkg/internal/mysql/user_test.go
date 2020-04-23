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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
		cfg *Config
		qr  *fake.QueryRunner
	)

	BeforeEach(func() {
		qr = fake.NewQueryRunner(false)

		cfg = &Config{
			User:        "root",
			Password:    "root-pwd",
			Host:        "host",
			Port:        3306,
			QueryRunner: qr.Run,
		}
	})

	Context("user related", func() {
		var (
			user            string
			pwd             string
			allowedHost     string
			permissions     []mysqlv1alpha1.MySQLPermission
			resourceOptions mysqlv1alpha1.AccountResourceLimits
		)

		BeforeEach(func() {
			user = "mysqlusername"
			pwd = "random-password"
			allowedHost = "localhost"
			permissions = []mysqlv1alpha1.MySQLPermission{
				{
					Schema:      "test_db",
					Tables:      []string{"*"},
					Permissions: []string{"PERM1", "PERM2"},
				},
			}
			resourceOptions = mysqlv1alpha1.AccountResourceLimits{
				"MAX_USER_CONNECTIONS": 10,
			}
		})

		It("should build the right queries for user creation", func() {
			assertQuery(qr,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ? WITH ?=?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHost, pwd, user, pwd, allowedHost, "MAX_USER_CONNECTIONS", 10, user, allowedHost,
			)
			Expect(CreateUserIfNotExists(cfg, user, pwd, allowedHost, permissions, resourceOptions)).To(Succeed())
		})

		It("should build queries with no resource limits", func() {
			assertQuery(qr,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHost, pwd, user, pwd, allowedHost, user, allowedHost,
			)

			Expect(CreateUserIfNotExists(cfg, user, pwd, allowedHost, permissions, nil)).To(Succeed())
		})

		It("should build queries with more resource limits", func() {
			resourceOptions["MAX_QUERIES_PER_HOUR"] = 100
			assertQuery(qr,
				strings.Join([]string{
					"BEGIN;\n",
					"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
					"ALTER USER ?@? IDENTIFIED BY ? WITH ?=? ?=?;\n",
					"GRANT PERM1, PERM2 ON `test_db`.* TO ?@?;\n",
					"COMMIT;",
				}, ""),
				user, allowedHost, pwd, user, pwd, allowedHost, "MAX_USER_CONNECTIONS", 10, "MAX_QUERIES_PER_HOUR", 100, user, allowedHost,
			)

			Expect(CreateUserIfNotExists(cfg, user, pwd, allowedHost, permissions, resourceOptions)).To(Succeed())
		})
	})

})

func assertQuery(qr *fake.QueryRunner, expectedQuery string, expectedArgs ...interface{}) {
	expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
		defer GinkgoRecover()

		Expect(query).To(Equal(expectedQuery))
		Expect(args).To(ConsistOf(expectedArgs...))

		return nil
	}

	qr.AddExpectedCalls(expectedQueryRunnerCall)
}
