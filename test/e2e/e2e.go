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
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	runtimeutils "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	core "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/bitpoke/mysql-operator/test/e2e/framework"
	"github.com/bitpoke/mysql-operator/test/e2e/framework/ginkgowrapper"
	pf "github.com/bitpoke/mysql-operator/test/e2e/framework/portforward"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	operatorNamespace = "mysql-operator"
	releaseName       = "operator"

	orchestratorPort = 3000
)

var orcTunnel *pf.Tunnel

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	kubeCfg, err := framework.LoadConfig()
	gomega.Expect(err).To(gomega.Succeed())
	restClient := core.NewForConfigOrDie(kubeCfg).RESTClient()

	c, err := client.New(kubeCfg, client.Options{})
	if err != nil {
		ginkgo.Fail(fmt.Sprintf("can't instantiate k8s client: %s", err))
	}

	// ginkgo node 1
	ginkgo.By("Install operator")
	operatorNsObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorNamespace,
		},
	}
	if err := c.Create(context.TODO(), operatorNsObj); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			ginkgo.Fail(fmt.Sprintf("can't create mysql-operator namespace: %s", err))
		}
	}
	framework.HelmInstallChart(releaseName, operatorNamespace)

	// Create a tunnel, port-forward orchestrator port to local port
	ginkgo.By("Port-forward orchestrator")
	orcTunnel = pf.NewTunnel(restClient, kubeCfg, operatorNamespace,
		fmt.Sprintf("%s-mysql-operator-0", releaseName),
		orchestratorPort,
	)
	if err := orcTunnel.ForwardPort(); err != nil {
		ginkgo.Fail(fmt.Sprintf("Fail to set port forwarding to orchestrator: %s", err))
	}

	// set orchestrator port to chossen port by tunnel
	framework.OrchestratorPort = orcTunnel.Local

	return nil

}, func(data []byte) {
	// all other nodes
	framework.Logf("Running BeforeSuite actions on all node")
})

// Similar to SynchornizedBeforeSuite, we want to run some operations only once (such as collecting cluster logs).
// Here, the order of functions is reversed; first, the function which runs everywhere,
// and then the function that only runs on the first Ginkgo node.
var _ = ginkgo.SynchronizedAfterSuite(func() {
	// Run on all Ginkgo nodes
	framework.Logf("Running AfterSuite actions on all node")
	framework.RunCleanupActions()

	// stop port-forwarding just if was started
	if orcTunnel != nil {
		ginkgo.By("Stop port-forwarding orchestrator")
		orcTunnel.Close()
	}

	// get the kubernetes client
	kubeCfg, err := framework.LoadConfig()
	gomega.Expect(err).To(gomega.Succeed())

	client, err := clientset.NewForConfig(kubeCfg)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Remove operator release")
	framework.HelmPurgeRelease(releaseName, operatorNamespace)

	ginkgo.By("Delete operator namespace")

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
	config := types.NewDefaultSuiteConfig()
	config.EmitSpecProgress = true
	config.RandomizeAllSpecs = true

	runtimeutils.ReallyCrash = true

	gomega.RegisterFailHandler(ginkgowrapper.Fail)
	// Disable skipped tests unless they are explicitly requested.
	if len(config.FocusStrings) == 0 && len(config.SkipStrings) == 0 {
		config.SkipStrings = []string{`\[Flaky\]`, `\[Feature:.+\]`}
	}

	// rps := func() (rps []ginkgo.Reporter) {
	// 	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	// 	if framework.TestContext.ReportDir != "" {
	// 		// TODO: we should probably only be trying to create this directory once
	// 		// rather than once-per-Ginkgo-node.
	// 		if err := os.MkdirAll(framework.TestContext.ReportDir, 0755); err != nil {
	// 			glog.Errorf("Failed creating report directory: %v", err)
	// 			return
	// 		}
	// 		// add junit report
	// 		rps = append(rps, reporters.NewJUnitReporter(path.Join(framework.TestContext.ReportDir, fmt.Sprintf("junit_%v%02d.xml", "mysql_o_", config.ParallelProcess))))

	// 		// add logs dumper
	// 		if framework.TestContext.DumpLogsOnFailure {
	// 			rps = append(rps, NewLogsPodReporter(
	// 				operatorNamespace,
	// 				path.Join(framework.TestContext.ReportDir, fmt.Sprintf("pods_logs_%d_%d.txt", config.RandomSeed, config.ParallelNode))))
	// 		}
	// 	} else {
	// 		// if reportDir is not specified then print logs to stdout
	// 		if framework.TestContext.DumpLogsOnFailure {
	// 			rps = append(rps, NewLogsPodReporter(operatorNamespace, ""))
	// 		}
	// 	}
	// 	return
	// }()

	glog.Infof("Starting e2e run on Ginkgo node %d", config.ParallelProcess)

	ginkgo.RunSpecs(t, "MySQL Operator E2E Suite")
}
