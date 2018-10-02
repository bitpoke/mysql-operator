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

package framework

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	core "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/mysql-operator/pkg/apis"
	orc "github.com/presslabs/mysql-operator/pkg/orchestrator"
)

const (
	maxKubectlExecRetries           = 5
	DefaultNamespaceDeletionTimeout = 10 * time.Minute
	orchestratorURITemplate         = "http://localhost:%d/api"
)

var OrchestratorPort = 3000

type Framework struct {
	BaseName  string
	Namespace *core.Namespace

	Client    client.Client
	ClientSet clientset.Interface

	cleanupHandle         CleanupActionHandle
	SkipNamespaceCreation bool

	OrcClient orc.Interface

	Timeout time.Duration
}

func NewFramework(baseName string) *Framework {
	By(fmt.Sprintf("Creating framework with timeout: %v", TestContext.TimeoutSeconds))
	f := &Framework{
		BaseName:              baseName,
		SkipNamespaceCreation: false,
	}

	BeforeEach(f.BeforeEach)
	AfterEach(f.AfterEach)

	return f
}

// BeforeEach gets a client and makes a namespace.
func (f *Framework) BeforeEach() {
	// The fact that we need this feels like a bug in ginkgo.
	// https://github.com/onsi/ginkgo/issues/222
	f.cleanupHandle = AddCleanupAction(f.AfterEach)
	f.Timeout = time.Duration(TestContext.TimeoutSeconds) * time.Second

	By("creating a kubernetes client")
	cfg, err := LoadConfig()
	Expect(err).NotTo(HaveOccurred())

	apis.AddToScheme(scheme.Scheme)

	f.Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	f.ClientSet, err = clientset.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	if !f.SkipNamespaceCreation {
		By("create a namespace api object")
		namespace, err := f.CreateNamespace(map[string]string{
			"e2e-framework": f.BaseName,
		})
		Expect(err).NotTo(HaveOccurred())

		f.Namespace = namespace
	}

	By("create a orchestrator client")
	f.OrcClient = orc.NewFromURI(fmt.Sprintf(orchestratorURITemplate, OrchestratorPort))

}

// AfterEach deletes the namespace, after reading its events.
func (f *Framework) AfterEach() {
	By("Collecting logs")
	if CurrentGinkgoTestDescription().Failed && TestContext.DumpLogsOnFailure {
		logFunc := Logf
		// TODO: log in file if ReportDir is set
		LogPodsWithLabels(f.ClientSet, f.Namespace.Name, map[string]string{}, logFunc)

	}

	By("Run cleanup actions")
	RemoveCleanupAction(f.cleanupHandle)

	By("Delete testing namespace")
	err := DeleteNS(f.ClientSet, f.Namespace.Name, DefaultNamespaceDeletionTimeout)
	if err != nil {
		Failf(fmt.Sprintf("Can't delete namespace: %s", err))
	}
}

func (f *Framework) CreateNamespace(labels map[string]string) (*core.Namespace, error) {
	return CreateTestingNS(f.BaseName, f.ClientSet, labels)
}

// WaitForPodReady waits for the pod to flip to ready in the namespace.
func (f *Framework) WaitForPodReady(podName string) error {
	return waitTimeoutForPodReadyInNamespace(f.ClientSet, podName,
		f.Namespace.Name, PodStartTimeout)
}
