/*
Copyright 2019 Pressinfra SRL
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

// nolint: errcheck, unparam
package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/drewolson/testflight"
	"github.com/pivotal-cf/brokerapi"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

const apiVersion = "2.14"

var _ = Describe("Service broker test", func() {
	var (
		stop chan struct{}
		// controller k8s client
		c client.Client

		brokerAPI http.Handler

		credentials = brokerapi.BrokerCredentials{
			Username: "username",
			Password: "password",
		}
	)

	makeProvisionRequest := func(id string, details map[string]interface{}, queryString string) *testflight.Response {
		response := &testflight.Response{}

		testflight.WithServer(brokerAPI, func(r *testflight.Requester) {
			path := "/v2/service_instances/" + id + queryString

			buffer := &bytes.Buffer{}
			json.NewEncoder(buffer).Encode(details)
			request, err := http.NewRequest("PUT", path, buffer)
			Expect(err).NotTo(HaveOccurred())

			request.Header.Add("Content-Type", "application/json")
			request.Header.Add("X-Broker-API-Version", apiVersion)
			request.SetBasicAuth(credentials.Username, credentials.Password)

			response = r.Do(request)
		})
		return response
	}

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		sb := &serviceBroker{Client: c}
		brokerLogger := lagertest.NewTestLogger("broker-api")
		brokerAPI = brokerapi.New(sb, brokerLogger, credentials)

		stop = startTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
	})

	It("should provision the cluster and it's secret", func() {
		ns := "default"
		name := "test-cluster-name"
		iid := "cb5b120c-936e-11e9-bc42-526af7764f64"
		r := makeProvisionRequest(iid, fakeProvisionDetails(ns, name), "")
		Expect(r.StatusCode).To(Equal(202))

		list := &api.MysqlClusterList{}
		Expect(c.List(context.TODO(), &client.ListOptions{}, list)).To(Succeed())
		Expect(list.Items).To(ConsistOf(clusterWith(ns, name, map[string]string{instanceIDLabel: iid}, nil)))

		sKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-credentials", name),
			Namespace: ns,
		}
		sec := &corev1.Secret{}
		Expect(c.Get(context.TODO(), sKey, sec)).To(Succeed())
	})

})

func fakeProvisionDetails(ns, name string) map[string]interface{} {
	return map[string]interface{}{
		"service_id":        MysqlServiceID,
		"plan_id":           DefaultPlanID,
		"organization_guid": "organization-guid",
		"space_guid":        "space-guid",
		"context": map[string]string{
			"namespace": ns,
		},
		"parameters": map[string]string{
			"name": name,
		},
	}
}

func clusterWith(ns, name string, labels, annotations map[string]string) gomegatypes.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Namespace":   Equal(ns),
			"Name":        Equal(name),
			"Labels":      Equal(labels),
			"Annotations": Equal(annotations),
		}),
	})
}
