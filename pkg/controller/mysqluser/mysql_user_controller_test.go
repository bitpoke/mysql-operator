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
		fakeQueryRunner *fake.QueryRunner
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

		fakeQueryRunner = fake.NewQueryRunner(false)

		var recFn reconcile.Reconciler
		recFn, requests = testutil.SetupTestReconcile(newReconciler(mgr, fakeQueryRunner.Run))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = testutil.StartTestManager(mgr)

	})

	AfterEach(func() {
		close(stop)
		fakeQueryRunner.DisallowExtraCalls()
	})

	var (
		expectedRequest reconcile.Request
		mySQLUser       *mysqluser.MySQLUser
		mySQLUserKey    client.ObjectKey
		cluster         *mysqlv1alpha1.MysqlCluster
		userPassword    string
	)

	When("creating the MySQLUser", func() {
		AfterEach(func() {
			// Cleanup resources
			Expect(c.Delete(context.TODO(), cluster)).To(Succeed())
			// Delete resource even if it's deleted or has finalizer on it
			c.Delete(context.TODO(), mySQLUser.Unwrap())
			if c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap()) == nil {
				mySQLUser.Finalizers = []string{}
				c.Update(context.TODO(), mySQLUser.Unwrap())
			}
			Eventually(func() error {
				return c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())
			}).ShouldNot(Succeed())
			fakeQueryRunner.AssertNoCallsLeft()
		})

		Context("without permissions", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(context.TODO(), c, &corev1.Secret{}),
					factories.CreateMySQLCluster(context.TODO(), c),
				)

				var err error
				userPassword, err = rand.AlphaNumericString(64)
				Expect(err).To(BeNil())

				mySQLUser = factories.BuildMySQLUser(c, cluster, factories.WithPassword(c, userPassword))
				mySQLUserKey = client.ObjectKey{Name: mySQLUser.Name, Namespace: mySQLUser.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      mySQLUser.Name,
					},
				}
			})

			It("reconciles the mysql user, but not before the finalizer has been set", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
					defer GinkgoRecover()

					Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
					Expect(meta.HasFinalizer(&mySQLUser.ObjectMeta, userFinalizer))

					Expect(dsn).To(Equal(expectedDSN))

					expectedQuery := strings.Join([]string{
						"BEGIN;\n",
						"CREATE USER IF NOT EXISTS ?@? IDENTIFIED BY ?;\n",
						"ALTER USER ?@? IDENTIFIED BY ?;\n",
						"COMMIT;",
					}, "")
					Expect(query).To(Equal(expectedQuery))

					Expect(args).To(ConsistOf(
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], userPassword,
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], userPassword,
					))

					return nil
				}

				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Create(context.TODO(), mySQLUser.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to true", func() {
				fakeQueryRunner.AllowExtraCalls()

				Expect(c.Create(context.TODO(), mySQLUser.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
				cond, exists := mySQLUser.ConditionExists(mysqlv1alpha1.MySQLUserReady)
				Expect(exists).To(BeTrue())
				Expect(cond.Status).To(Equal(corev1.ConditionTrue))
			})
		})

		Context("with permissions", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(context.TODO(), c, &corev1.Secret{}),
					factories.CreateMySQLCluster(context.TODO(), c),
				)

				var err error
				userPassword, err = rand.AlphaNumericString(64)
				Expect(err).To(BeNil())

				mySQLUser = factories.BuildMySQLUser(
					c, cluster,
					factories.WithPassword(c, userPassword),
					factories.WithPermissions(
						mysqlv1alpha1.MySQLPermission{
							Permissions: []string{
								"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER",
							},
							Tables: []string{"*"},
							Schema: "db",
						},
						mysqlv1alpha1.MySQLPermission{
							Schema:      "sys_operator",
							Tables:      []string{"heartbeat", "eyeblink"},
							Permissions: []string{"SELECT", "CREATE"},
						},
					),
				)
				mySQLUserKey = client.ObjectKey{Name: mySQLUser.Name, Namespace: mySQLUser.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      mySQLUser.Name,
					},
				}
			})

			It("reconciles the mysql user, but not before the finalizer has been set", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
					defer GinkgoRecover()

					Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
					Expect(meta.HasFinalizer(&mySQLUser.ObjectMeta, userFinalizer))

					Expect(dsn).To(Equal(expectedDSN))

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
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], userPassword, // create user
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], userPassword, // alter user
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], // grant privilege #1
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], // grant privilege #2
						mySQLUser.Spec.User, mySQLUser.Spec.AllowedHosts[0], // grant privilege #3
					))

					return nil
				}

				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Create(context.TODO(), mySQLUser.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to true", func() {
				fakeQueryRunner.AllowExtraCalls()

				Expect(c.Create(context.TODO(), mySQLUser.Unwrap())).To(Succeed())

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for reconciliation triggered by finalizer being set
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
				cond, exists := mySQLUser.ConditionExists(mysqlv1alpha1.MySQLUserReady)
				Expect(exists).To(BeTrue())
				Expect(cond.Status).To(Equal(corev1.ConditionTrue))
			})
		})

		Context("and the user cannot be created in mysql", func() {
			BeforeEach(func() {
				// Create prerequisite resources
				cluster = factories.NewMySQLCluster(
					factories.CreateMySQLClusterSecret(context.TODO(), c, &corev1.Secret{}),
					factories.CreateMySQLCluster(context.TODO(), c),
				)

				var err error
				userPassword, err = rand.AlphaNumericString(64)
				Expect(err).To(BeNil())

				// The mysql user creation fails
				expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
					defer GinkgoRecover()

					return errors.New("couldn't create user")
				}
				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall, expectedQueryRunnerCall)

				mySQLUser = factories.CreateMySQLUser(c, cluster, factories.WithPassword(c, userPassword))
				mySQLUserKey = client.ObjectKey{Name: mySQLUser.Name, Namespace: mySQLUser.Namespace}
				expectedRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "default",
						Name:      mySQLUser.Name,
					},
				}
			})

			It("doesn't remove the user finalizer and it tries to reconcile again", func() {
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Check that the finalizer is still there
				Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
				Expect(meta.HasFinalizer(&mySQLUser.ObjectMeta, userFinalizer)).To(BeTrue())

				// Wait for second reconciliation, since the first one failed
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))
			})
			It("sets the ready status to false", func() {
				fakeQueryRunner.AllowExtraCalls()

				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation, since the first one failed
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))

				// check status
				Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
				cond, exists := mySQLUser.ConditionExists(mysqlv1alpha1.MySQLUserReady)
				Expect(exists).To(BeTrue())
				Expect(cond.Status).To(Equal(corev1.ConditionFalse))
				Expect(cond.Reason).To(Equal("ProvisionFailed"))
			})
		})
	})
	When("the resource is being deleted", func() {
		BeforeEach(func() {
			// Create prerequisite resources
			cluster = factories.NewMySQLCluster(
				factories.CreateMySQLClusterSecret(context.TODO(), c, &corev1.Secret{}),
				factories.CreateMySQLCluster(context.TODO(), c),
			)

			var err error
			userPassword, err = rand.AlphaNumericString(64)
			Expect(err).To(BeNil())

			mySQLUser = factories.CreateMySQLUser(c, cluster, factories.WithPassword(c, userPassword),
				func(user *mysqluser.MySQLUser) error {
					meta.AddFinalizer(&user.ObjectMeta, userFinalizer)

					return nil
				})
			mySQLUserKey = client.ObjectKey{Name: mySQLUser.Name, Namespace: mySQLUser.Namespace}
			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      mySQLUser.Name,
				},
			}

			// Allow creation reconciliation to proceed
			allowReconciliation(fakeQueryRunner, requests, expectedRequest)
		})

		AfterEach(func() {
			// Cleanup resources
			err := c.Delete(context.TODO(), cluster)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
			fakeQueryRunner.AssertNoCallsLeft()
		})

		Context("and the user can be deleted in mysql", func() {
			var deletionResult error = nil

			It("deletes the mysql user", func() {
				expectedDSN := "root:password@tcp(" + cluster.Name + "-mysql-master." + cluster.Namespace + ":3306)" +
					"/?timeout=5s&multiStatements=true&interpolateParams=true"
				expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
					defer GinkgoRecover()

					Expect(dsn).To(Equal(expectedDSN))

					expectedQuery := "DROP USER IF EXISTS ?;"
					Expect(query).To(Equal(expectedQuery))

					Expect(args).To(ConsistOf(mySQLUser.Spec.User))

					return deletionResult
				}

				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Delete(context.TODO(), mySQLUser.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation triggered by finalizer removal
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
			})

			It("removes the user finalizer, and the resource is deleted", func() {
				fakeQueryRunner.AllowExtraCalls()

				Expect(c.Delete(context.TODO(), mySQLUser.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Wait for second reconciliation triggered by finalizer removal
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// We need to make sure that the controller does not create infinite loops
				Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))

				err := c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})
		Context("and the user cannot be deleted in mysql", func() {
			deletionResult := errors.New("couldn't delete user")

			AfterEach(func() {
				// Delete resource even if it's deleted or has finalizer on it
				c.Delete(context.TODO(), mySQLUser.Unwrap())
				if c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap()) == nil {
					mySQLUser.Finalizers = []string{}
					c.Update(context.TODO(), mySQLUser.Unwrap())
				}
				Eventually(func() error {
					return c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())
				}).ShouldNot(Succeed())
			})

			It("doesn't remove the user finalizer and it tries to reconcile again", func() {
				expectedQueryRunnerCall := func(dsn string, query string, args ...interface{}) error {
					defer GinkgoRecover()

					return deletionResult
				}
				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)

				Expect(c.Delete(context.TODO(), mySQLUser.Unwrap())).To(Succeed())
				// Wait for initial reconciliation
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

				// Check that the finalizer is still there
				Expect(c.Get(context.TODO(), mySQLUserKey, mySQLUser.Unwrap())).To(Succeed())
				Expect(meta.HasFinalizer(&mySQLUser.ObjectMeta, userFinalizer)).To(BeTrue())

				// Wait for second reconciliation, since the first one failed
				fakeQueryRunner.AddExpectedCalls(expectedQueryRunnerCall)
				Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))
			})
		})

		Context("and the MySQL cluster has been deleted", func() {
			BeforeEach(func() {
				Expect(c.Delete(context.TODO(), cluster)).To(Succeed())
			})
			It("assumes the user has been deleted", func() {
				Expect(c.Delete(context.TODO(), mySQLUser.Unwrap())).To(Succeed())
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

func allowReconciliation(fakeQueryRunner *fake.QueryRunner, requests chan reconcile.Request, expectedRequest reconcile.Request) {
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
