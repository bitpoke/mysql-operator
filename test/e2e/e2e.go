/*
Copyright 2015 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiext_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	runtimeutils "k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
	"github.com/presslabs/mysql-operator/test/e2e/framework"
	"github.com/presslabs/mysql-operator/test/e2e/framework/ginkgowrapper"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	operatorNamespace = "mysql-operator"
	releaseName       = "operator"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	ginkgo.By("Create operator namespace")
	// client, _, err := framework.KubernetesClients()
	// gomega.Expect(err).NotTo(gomega.HaveOccurred())
	// _, err = framework.CreateTestingNS(operatorNamespace, client, map[string]string{})
	// gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Install operator")
	HelmInstallChart(releaseName)

	waitForCRDs()
	// ginkgo node 1
	return nil

}, func(data []byte) {
	// all other nodes
})

// wait for crds to be ready, crds are installed by operator
func waitForCRDs() {
	kubeCfg, err := framework.LoadConfig()
	gomega.Expect(err).To(gomega.Succeed())

	crdcl, err := apiext_clientset.NewForConfig(kubeCfg)
	gomega.Expect(err).To(gomega.Succeed())

	crds := []*apiext.CustomResourceDefinition{
		api.ResourceMysqlClusterCRD,
		api.ResourceMysqlBackupCRD,
	}
	for _, crd := range crds {
		gomega.Eventually(func() error {
			return kube.WaitForCRD(crdcl, crd)
		}, 30*time.Second, 2*time.Second).Should(gomega.Succeed())
	}
}

// Similar to SynchornizedBeforeSuite, we want to run some operations only once (such as collecting cluster logs).
// Here, the order of functions is reversed; first, the function which runs everywhere,
// and then the function that only runs on the first Ginkgo node.
var _ = ginkgo.SynchronizedAfterSuite(func() {
	// Run on all Ginkgo nodes
	framework.Logf("Running AfterSuite actions on all node")
	framework.RunCleanupActions()

	ginkgo.By("Remove operator release")
	HelmDeleteRelease(releaseName)

	ginkgo.By("Delete operator namespace")
	client, _, err := framework.KubernetesClients()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	if err := framework.DeleteNS(client, operatorNamespace, framework.DefaultNamespaceDeletionTimeout); err != nil {
		framework.Failf(fmt.Sprintf("Can't delete namespace: %s", err))
	}

}, func() {
	// Run only Ginkgo on node 1
	framework.Logf("Running AfterSuite actions on node 1")

})

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
// If a "report directory" is specified, one or more JUnit test reports will be
// generated in this directory, and cluster logs will also be saved.
// This function is called on each Ginkgo node in parallel mode.
func RunE2ETests(t *testing.T) {
	runtimeutils.ReallyCrash = true

	gomega.RegisterFailHandler(ginkgowrapper.Fail)
	// Disable skipped tests unless they are explicitly requested.
	if config.GinkgoConfig.FocusString == "" && config.GinkgoConfig.SkipString == "" {
		config.GinkgoConfig.SkipString = `\[Flaky\]|\[Feature:.+\]`
	}

	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	var r []ginkgo.Reporter
	if framework.TestContext.ReportDir != "" {
		// TODO: we should probably only be trying to create this directory once
		// rather than once-per-Ginkgo-node.
		if err := os.MkdirAll(framework.TestContext.ReportDir, 0755); err != nil {
			glog.Errorf("Failed creating report directory: %v", err)
		} else {
			r = append(r, reporters.NewJUnitReporter(path.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%v%02d.xml", "mysql_o_", config.GinkgoConfig.ParallelNode))))
		}
	}
	glog.Infof("Starting e2e run on Ginkgo node %d", config.GinkgoConfig.ParallelNode)

	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Mysql operator e2e suite", r)
}
