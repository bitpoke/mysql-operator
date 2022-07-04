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

package mysqldatabase

import (
	"context"
	"fmt"
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mysqlv1alpha1 "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/controller/internal/testutil"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysql/fake"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqldatabase"
	"github.com/bitpoke/mysql-operator/pkg/testutil/factories"
	gm "github.com/bitpoke/mysql-operator/pkg/testutil/gomegamatcher"
)

var _ = Describe("MySQL database controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request

		// controller k8s client
		c client.Client

		fakeQR *fake.SQLRunner

		ctxCancel func()
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

		fakeQR = fake.NewQueryRunner(false)

		var recFn reconcile.Reconciler
		rec := newReconciler(mgr, fake.NewFakeFactory(fakeQR)).(*ReconcileMySQLDatabase)
		// inject an uncached client
		rec.Client = c
		recFn, requests = testutil.SetupTestReconcile(rec)
		Expect(add(mgr, recFn)).To(Succeed())

		_, ctxCancel = testutil.StartTestManager(mgr)
	})

	AfterEach(func() {
		ctxCancel()
	})

	When("db resource is created", func() {
		var (
			clusterName     string
			db              *mysqldatabase.Database
			expectedRequest reconcile.Request
		)
		BeforeEach(func() {
			clusterName = fmt.Sprintf("mysql-%d", rand.Int())
			db = factories.NewDatabase(factories.WithMySQLCluster(context.TODO(), c, clusterName))
			dbName := db.Name

			fakeQR.AddExpectedCalls(
				func(query string, args ...interface{}) error {
					By("Creating the database")
					Expect(query).To(Equal(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName)))
					Expect(args).To(BeEmpty())

					return nil
				},
				func(query string, args ...interface{}) error {
					return nil
				},
			)

			// create DB in k8s
			Expect(c.Create(context.TODO(), db.Unwrap())).To(Succeed())

			expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}

			// first event when the resource is created
			Eventually(requests).Should(Receive(Equal(expectedRequest)))

			// second event when is updated with the finalizer and status
			Eventually(requests, "2s").Should(Receive(Equal(expectedRequest)))

			// should not receive any new reconcile requests
			Consistently(requests).ShouldNot(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			fakeQR.AssertNoCallsLeft()
			fakeQR.AllowExtraCalls()
			forceDeleteDb(c, db)
		})

		It("should set the finalizer", func() {
			// refresh resource
			Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).To(Succeed())
			Expect(db.Finalizers).To(ContainElement(mysqlPreventDeletionFinalizer))
		})

		It("should mark the resource ready", func() {
			// refresh resource
			Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).To(Succeed())
			Expect(db.Unwrap()).To(gm.HaveCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionTrue))

		})

		Context("and when the resource is deleted", func() {
			It("should not delete the db if query returns error", func() {
				fakeQR.AddExpectedCalls(
					func(query string, args ...interface{}) error {
						return fmt.Errorf("fake db connection error")
					},
				)
				// delete the resource
				Expect(c.Delete(context.TODO(), db.Unwrap())).To(Succeed())

				// a reconcile request - that will fail
				Eventually(requests).Should(Receive(Equal(expectedRequest)))

				// expect the db still exists and finalizer still set
				Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).To(Succeed())
				Expect(db.Finalizers).To(ContainElement(mysqlPreventDeletionFinalizer))
			})
			It("should drop the database when deleted ", func() {
				fakeQR.AddExpectedCalls(
					func(query string, args ...interface{}) error {
						By("Deleting the database")
						//Expect(dsn).To(Equal(getExpectedDSN(clusterName)))
						Expect(query).To(Equal(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", db.Spec.Database)))

						return nil
					},
				)

				// delete the database
				Expect(c.Delete(context.TODO(), db.Unwrap())).To(Succeed())

				// expect the reconcile event to delete db and remove finalizer
				Eventually(requests).Should(Receive(Equal(expectedRequest)))

				// the database should be gone
				Eventually(func() error {
					return c.Get(context.TODO(), dbObjKey(db), db.Unwrap())
				}).ShouldNot(Succeed())

				// last event but already deleted
				Eventually(requests).Should(Receive(Equal(expectedRequest)))
			})
			It("should succeed if the cluster has been deleted", func() {
				// delete the cluster
				cluster := &mysqlv1alpha1.MysqlCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName, Namespace: db.Namespace,
					},
				}
				Expect(c.Delete(context.TODO(), cluster)).To(Succeed())

				// delete the database
				Expect(c.Delete(context.TODO(), db.Unwrap())).To(Succeed())

				// expect the reconcile event to delete db and remove finalizer
				Eventually(requests).Should(Receive(Equal(expectedRequest)))

				// the database resource should be gone
				Eventually(func() error {
					return c.Get(context.TODO(), dbObjKey(db), db.Unwrap())
				}).ShouldNot(Succeed())
			})
			It("should delete resource and database should retain", func() {
				fakeQR.AddExpectedCalls(
					func(query string, args ...interface{}) error {
						defer GinkgoRecover()

						PanicWith("Shouldn't call this!")
						return nil
					},
				)

				if db.ObjectMeta.Annotations == nil {
					db.ObjectMeta.Annotations = map[string]string{}
				}
				db.ObjectMeta.Annotations[mysqlv1alpha1.MysqlResourceDeletionPolicyAnnotationKey] = string(mysqlv1alpha1.MysqlResourceDeletionPolicyRetain)

				// delete the resource
				Expect(c.Delete(context.TODO(), db.Unwrap())).To(Succeed())

				// expect the reconcile event to delete db and remove finalizer
				Eventually(requests).Should(Receive(Equal(expectedRequest)))

				// the database should be gone
				Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).ShouldNot(Succeed())

				// last event but already deleted
				Eventually(requests).Should(Receive(Equal(expectedRequest)))

				delete(db.ObjectMeta.Annotations, mysqlv1alpha1.MysqlResourceDeletionPolicyAnnotationKey)
			})
		})
	})

	It("should run the correct queries", func() {
		clusterName := fmt.Sprintf("mysql-%d", rand.Int())
		db := factories.NewDatabase(factories.WithMySQLCluster(context.TODO(), c, clusterName))
		dbName := db.Name

		fakeQR.AddExpectedCalls(
			func(query string, args ...interface{}) error {
				defer GinkgoRecover()

				By("Creating the database")
				Expect(query).To(Equal(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName)))

				return nil
			},
			func(query string, args ...interface{}) error {
				defer GinkgoRecover()

				By("Creating the database second run")
				Expect(query).To(Equal(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName)))

				return nil
			},
		)

		Expect(c.Create(context.TODO(), db.Unwrap())).To(Succeed())
		expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}

		Eventually(requests).Should(Receive(Equal(expectedRequest)))
		Eventually(requests).Should(Receive(Equal(expectedRequest)))

		fakeQR.AllowExtraCalls()
		forceDeleteDb(c, db)
	})

	It("should fail if the cluster doesn't exists", func() {
		db := factories.NewDatabase()

		Expect(c.Create(context.TODO(), db.Unwrap())).To(Succeed())
		expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}

		// first reconcile event, use a timeout of 2 seconds to catch the reconcile event even if it's delayed
		Eventually(requests).Should(Receive(Equal(expectedRequest)))

		Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).To(Succeed())
		Expect(db.Unwrap()).To(gm.HaveCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionFalse))

		Expect(c.Delete(context.TODO(), db.Unwrap())).To(Succeed())

		fakeQR.AllowExtraCalls()
		forceDeleteDb(c, db)
	})

	It("should update status", func() {
		db := factories.NewDatabase()

		db.UpdateCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionTrue, mysqldatabase.ProvisionSucceeded, "success")
		Expect(db.Unwrap()).To(gm.HaveCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionTrue))

		db.UpdateCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionFalse, mysqldatabase.ProvisionFailed, "error")
		Expect(db.Unwrap()).To(gm.HaveCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionFalse))
	})

	It("should not reconcile resource across namespaces", func() {
		Expect(c.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other",
			},
		})).To(Succeed())

		db := factories.NewDatabase(func(db *mysqldatabase.Database) error {
			db.Namespace = "other"
			return nil
		})

		Expect(c.Create(context.TODO(), db.Unwrap())).To(Succeed())
		expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}

		Eventually(requests).Should(Receive(Equal(expectedRequest)))

		Expect(c.Get(context.TODO(), dbObjKey(db), db.Unwrap())).To(Succeed())
		Expect(db.Unwrap()).To(gm.HaveCondition(mysqlv1alpha1.MysqlDatabaseReady, corev1.ConditionFalse))

		fakeQR.AllowExtraCalls()
		forceDeleteDb(c, db)
	})
})

func dbObjKey(db *mysqldatabase.Database) client.ObjectKey {
	return types.NamespacedName{
		Name:      db.Name,
		Namespace: db.Namespace,
	}
}

func forceDeleteDb(c client.Client, db *mysqldatabase.Database) {
	// delete resource even if it's deleted or has finalizer on it
	c.Delete(context.TODO(), db.Unwrap())
	if c.Get(context.TODO(), dbObjKey(db), db.Unwrap()) == nil {
		db.Finalizers = []string{}
		c.Update(context.TODO(), db.Unwrap())
	}
	Eventually(func() error {
		return c.Get(context.TODO(), dbObjKey(db), db.Unwrap())
	}).ShouldNot(Succeed())
}
