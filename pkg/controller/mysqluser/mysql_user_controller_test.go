/*
Copyright 2020 Pressinfra SRL.

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

package mysqluser

import (
	"context"
	"errors"
	"github.com/presslabs/mysql-operator/pkg/testutil/factories"
	"github.com/presslabs/mysql-operator/pkg/testutil/gomegamatcher"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/presslabs/controller-util/meta"
	"github.com/presslabs/controller-util/rand"

	mysqlv1alpha1 "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/controller/internal/testutil"
	"github.com/presslabs/mysql-operator/pkg/internal/mysql/fake"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqluser"
)

const timeout = time.Second * 1

var _ = Describe("MySQL user controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		// fake query runner
		fakeSQL *fake.SQLRunner
	)

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{
			Scheme:             scheme.Scheme,
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		// create new k8s client
		// NOTE: create a new k8s client without cache to have more stable tests
		c, err = client.New(cfg, client.Options{})
		Expect(err).To(Succeed())

		fakeSQL = fake.NewQueryRunner(false)

		var recFn reconcile.Reconciler
		recFn, requests = testutil.SetupTestReconcile(newReconciler(mgr, fake.NewFakeFactory(fakeSQL)))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = testutil.StartTestManager(mgr)

	})

	AfterEach(func() {
		close(stop)
		fakeSQL.DisallowExtraCalls()
	})

	var (
		expectedRequest reconcile.Request
		user            *mysqluser.MySQLUser
		userKey         client.ObjectKey
		cluster         *mysqlv1alpha1.MysqlCluster
		userPassword    string
	)

	BeforeEach(func() {
		var err error
		userPassword, err = rand.AlphaNumericString(64)
		Expect(err).To(BeNil())
	})

	When("creating the MysqlUser", func() {
		AfterEach(func() {
			// Cleanup resources
			Expect(c.Delete(context.TODO(), cluster)).To(Succeed())
			// Delete resource even if it's deleted or has finalizer on it
			c.Delete(context.TODO(), user.Unwrap())
			if c.Get(context.TODO(), userKey, user.Unwrap()) == nil {
				user.Finalizers = []string{}
				c.Update(context.TODO(), user.Unwrap())
			}
			Eventually(func() error {
				return c.Get(context.TODO(), userKey, user.Unwrap())
			}).ShouldNot(Succeed())
			fakeSQL.AssertNoCallsLeft()
		})

		Context("without permissions", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(c, &corev1.Secret{}),
					factories.CreateMySQLClusterInK8s(c),
				)

				user = factories.MySQLUser(cluster, factories.WithPassword(c, userPassword))
				userKey = client.ObjectKey{Name: user.Name, Namespace: user.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      user.Name,
					},
				}
			})

			It("reconciles the mysql user, but not before the finalizer has been set", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Checking finalizer")
					Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
					Expect(meta.HasFinalizer(&user.ObjectMeta, userFinalizer))

					By("Creating user")

					expectedQuery := strings.Join([]string{
						"BEGIN;\n",
						"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
						"ALTER USER ?@? IDENTIFIED BY ?;\n",
						"COMMIT;",
					}, "")
					Expect(query).To(Equal(expectedQuery))

					Expect(args).To(ConsistOf(
						user.Spec.User, user.Spec.AllowedHosts[0], userPassword,
						user.Spec.User, user.Spec.AllowedHosts[0], userPassword,
					))

					return nil
				}

				fakeSQL.AssertDSN(expectedDSN)
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Create(context.TODO(), user.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to true", func() {
				fakeSQL.AllowExtraCalls()

				Expect(c.Create(context.TODO(), user.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Unwrap()).To(gomegamatcher.HaveCondition(mysqlv1alpha1.MySQLUserReady, corev1.ConditionTrue))
			})
		})

		Context("with permissions", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(c, &corev1.Secret{}),
					factories.CreateMySQLClusterInK8s(c),
				)

				user = factories.MySQLUser(
					cluster,
					factories.WithPassword(c, userPassword),
					factories.WithPermissions(
						mysqlv1alpha1.MysqlPermission{
							Permissions: []string{
								"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER",
							},
							Tables: []string{"*"},
							Schema: "db",
						},
						mysqlv1alpha1.MysqlPermission{
							Schema:      "sys_operator",
							Tables:      []string{"heartbeat", "eyeblink"},
							Permissions: []string{"SELECT", "CREATE"},
						},
					),
				)
				userKey = client.ObjectKey{Name: user.Name, Namespace: user.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      user.Name,
					},
				}
			})

			It("reconciles the mysql user, but not before the finalizer has been set", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Creating the user")
					expectedQuery := strings.Join([]string{
						"BEGIN;\n",
						"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
						"ALTER USER ?@? IDENTIFIED BY ?;\n",
						"GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, INDEX, ALTER ON `db`.* TO ?@?;\n",
						"GRANT SELECT, CREATE ON `sys_operator`.`heartbeat` TO ?@?;\n",
						"GRANT SELECT, CREATE ON `sys_operator`.`eyeblink` TO ?@?;\n",
						"COMMIT;",
					}, "")
					Expect(query).To(Equal(expectedQuery))

					Expect(args).To(ConsistOf(
						user.Spec.User, user.Spec.AllowedHosts[0], userPassword, // create user
						user.Spec.User, user.Spec.AllowedHosts[0], userPassword, // alter user
						user.Spec.User, user.Spec.AllowedHosts[0], // grant privilege #1
						user.Spec.User, user.Spec.AllowedHosts[0], // grant privilege #2
						user.Spec.User, user.Spec.AllowedHosts[0], // grant privilege #3
					))

					return nil
				}

				fakeSQL.AssertDSN(expectedDSN)
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Create(context.TODO(), user.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to true", func() {
				fakeSQL.AllowExtraCalls()

				Expect(c.Create(context.TODO(), user.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Unwrap()).To(gomegamatcher.HaveCondition(mysqlv1alpha1.MySQLUserReady, corev1.ConditionTrue))
			})
		})

		Context("user is created with multiple allowed hosts", func() {
			BeforeEach(func() {
				fakeSQL.AllowExtraCalls()

				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(c, &corev1.Secret{}),
					factories.CreateMySQLClusterInK8s(c),
				)

				user = factories.MySQLUser(cluster, factories.WithPassword(c, userPassword),
					func(user *mysqluser.MySQLUser) error {
						user.Spec.AllowedHosts = []string{"test1", "test2"}
						return nil
					},
					factories.CreateMySQLUserInK8s(c),
				)
				userKey = client.ObjectKey{Name: user.Name, Namespace: user.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      user.Name,
					},
				}

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})

			It("should update status with allowed hosts", func() {
				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Status.AllowedHosts).To(ConsistOf("test1", "test2"))
				// copy user name because is used in other goroutines
				// an may end up in a data-race
				userName := user.Spec.User

				fakeSQL.AddExpectedCalls(func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Updating the user with new allowed host (first reconciliation)")
					expectedQuery := strings.Join([]string{
						"BEGIN;\n",
						"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ?;\n",
						"ALTER USER ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ?;\n",
						"COMMIT;",
					}, "")

					Expect(query).To(Equal(expectedQuery))
					Expect(args).To(ConsistOf(
						userName, "test1", userPassword,
						userName, "new-host", userPassword,
						userName, "test1", userPassword,
						userName, "new-host", userPassword,
					))

					return nil
				}, func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Remove user old allowed host (first reconciliation)")
					Expect(query).To(Equal("DROP USER IF EXISTS ?@?;"))
					Expect(args).To(ConsistOf(userName, "test2"))
					return nil
				}, func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Updating the user with new allowed host (second reconciliation)")
					expectedQuery := strings.Join([]string{
						"BEGIN;\n",
						"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ?;\n",
						"ALTER USER ?@? IDENTIFIED BY ?, ?@? IDENTIFIED BY ?;\n",
						"COMMIT;",
					}, "")

					Expect(query).To(Equal(expectedQuery))
					Expect(args).To(ConsistOf(
						userName, "test1", userPassword,
						userName, "new-host", userPassword,
						userName, "test1", userPassword,
						userName, "new-host", userPassword,
					))
					return nil
				})
				fakeSQL.DisallowExtraCalls()

				user.Spec.AllowedHosts = []string{"test1", "new-host"}
				Expect(c.Update(context.TODO(), user.Unwrap())).To(Succeed())

				// updates the user
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				// status get updated (second reconciliation)
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			})
		})

		Context("and the user cannot be created in mysql", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(c, &corev1.Secret{}), factories.CreateMySQLClusterInK8s(c),
				)

				// The mysql user creation fails
				expectedQueryRunnerCall := func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					return errors.New("couldn't create user")
				}
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall, expectedQueryRunnerCall)

				user = factories.MySQLUser(cluster, factories.WithPassword(c, userPassword), factories.CreateMySQLUserInK8s(c))
				userKey = client.ObjectKey{Name: user.Name, Namespace: user.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      user.Name,
					},
				}
			})

			It("tries to reconcile again", func() {
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Check that the finalizer is still there
				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Finalizers).ToNot(ContainElement(userFinalizer))

				// Wait for second reconciliation, since the first one failed
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to false", func() {
				fakeSQL.AllowExtraCalls()

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation, since the first one failed
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))

				// check status
				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Unwrap()).To(gomegamatcher.HaveCondition(mysqlv1alpha1.MySQLUserReady, corev1.ConditionFalse))
			})
		})
	})
	When("the resource is being deleted", func() {
		BeforeEach(func() {
			// Create prerequisite resources
			cluster = factories.NewMySQLCluster(
				factories.CreateMySQLClusterSecret(c, &corev1.Secret{}),
				factories.CreateMySQLClusterInK8s(c),
			)

			user = factories.MySQLUser(cluster, factories.WithPassword(c, userPassword),
				func(user *mysqluser.MySQLUser) error {
					meta.AddFinalizer(&user.ObjectMeta, userFinalizer)

					return nil
				}, factories.CreateMySQLUserInK8s(c))
			userKey = client.ObjectKey{Name: user.Name, Namespace: user.Namespace}
			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      user.Name,
				},
			}

			// Allow creation reconciliation to proceed
			allowReconciliation(fakeSQL, requests, expectedRequest)
		})

		AfterEach(func() {
			// Cleanup resources
			err := c.Delete(context.TODO(), cluster)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
			fakeSQL.AssertNoCallsLeft()
		})

		Context("and the user can be deleted in mysql", func() {
			var deletionResult error = nil

			It("deletes the mysql user", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					By("Deleting the user")
					expectedQuery := "DROP USER IF EXISTS ?;"
					Expect(query).To(Equal(expectedQuery))
					Expect(args).To(ConsistOf(user.Spec.User))

					return deletionResult
				}

				fakeSQL.AssertDSN(expectedDSN)
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Delete(context.TODO(), user.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation triggered by finalizer removal
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})

			It("removes the user finalizer, and the resource is deleted", func() {
				fakeSQL.AllowExtraCalls()

				Expect(c.Delete(context.TODO(), user.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation triggered by finalizer removal
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				err := c.Get(context.TODO(), userKey, user.Unwrap())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})
		Context("and the user cannot be deleted in mysql", func() {
			deletionResult := errors.New("couldn't delete user")

			AfterEach(func() {
				// Delete resource even if it's deleted or has finalizer on it
				c.Delete(context.TODO(), user.Unwrap())
				if c.Get(context.TODO(), userKey, user.Unwrap()) == nil {
					user.Finalizers = []string{}
					c.Update(context.TODO(), user.Unwrap())
				}
				Eventually(func() error {
					return c.Get(context.TODO(), userKey, user.Unwrap())
				}).ShouldNot(Succeed())
			})

			It("doesn't remove the user finalizer and it tries to reconcile again", func() {
				expectedQueryRunnerCall := func(query string, args ...interface{}) error {
					defer GinkgoRecover()

					return deletionResult
				}
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Delete(context.TODO(), user.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Check that the finalizer is still there
				Expect(c.Get(context.TODO(), userKey, user.Unwrap())).To(Succeed())
				Expect(user.Finalizers).To(ContainElement(userFinalizer))

				// Wait for second reconciliation, since the first one failed
				fakeSQL.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))
			})
		})

		Context("and the MySQL cluster has been deleted", func() {
			BeforeEach(func() {
				Expect(c.Delete(context.TODO(), cluster)).To(Succeed())
			})
			It("assumes the user has been deleted", func() {
				Expect(c.Delete(context.TODO(), user.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation triggered by finalizer removal
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})
		})
	})
})

func allowReconciliation(fakeQueryRunner *fake.SQLRunner, requests chan reconcile.Request, expectedRequest reconcile.Request) {
	fakeQueryRunner.AllowExtraCalls()
	done := time.After(500 * time.Millisecond)
drain:
	for {
		select {
		case <-requests:
			continue
		case <-done:
			break drain
		}
	}
	Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

	fakeQueryRunner.DisallowExtraCalls()
}
