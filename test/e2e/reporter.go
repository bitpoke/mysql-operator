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
	"io"
	"os"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

type podLogReporter struct {
	podName   string
	namespace string
}

func NewLogsPodReporter(podName, ns string) reporters.Reporter {
	return &podLogReporter{
		podName:   podName,
		namespace: ns,
	}
}

// called when suite starts
func (r *podLogReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {
	fmt.Println("######### SpecSuiteWillBegin")
}

// called before BeforeSuite before starting tests
func (r *podLogReporter) BeforeSuiteDidRun(setupSummary *types.SetupSummary) {
	fmt.Println("######### BeforeSuiteDidRun")
}

// called before every test
func (r *podLogReporter) SpecWillRun(specSummary *types.SpecSummary) {
	fmt.Println("######### SpecWillRun")
}

// called after every test
func (r *podLogReporter) SpecDidComplete(specSummary *types.SpecSummary) {
	fmt.Println("######### SpecDidComplete")
	if specSummary.State <= types.SpecStatePassed {
		return
	}

	// get the kubernetes client
	kubeCfg, err := framework.LoadConfig()
	if err != nil {
		fmt.Println("Failed to get kubeconfig!")
		return
	}

	client, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		fmt.Println("Failed to create k8s client!")
		return
	}

	LogPodsWithLabels(client, r.namespace, map[string]string{}, specSummary.RunTime, os.Stdout)

}

// called before AfterSuite runs
func (r *podLogReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {
	fmt.Println("######### AfterSuiteDidRun")
	// skip printing logs if test is in pending, skipping or passed
	if setupSummary.State <= types.SpecStatePassed {
		return
	}
}

// caleed at the end
func (r *podLogReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {
	fmt.Println("######### SpecSuiteDidEnd")
	if summary.NumberOfFailedSpecs == 0 {
		return
	}

	// get the kubernetes client
	//  kubeCfg, err := framework.LoadConfig()
	//  if err != nil {
	//  	fmt.Println("Failed to get kubeconfig!")
	//  	return
	//  }

	//  client, err := clientset.NewForConfig(kubeCfg)
	//  if err != nil {
	//  	fmt.Println("Failed to create k8s client!")
	//  	return
	//  }

	//framework.LogPodsWithLabels(client, r.namespace, map[string]string{}, framework.Logf)

}

func LogPodsWithLabels(c clientset.Interface, ns string, match map[string]string, since time.Duration, out io.Writer) {
	podList, err := c.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: labels.SelectorFromSet(match).String()})
	if err != nil {
		fmt.Fprintf(out, "error listing pods: %s", err)
		return
	}

	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			fmt.Fprintf(out, "\n\n===============\nSTART LOGS for %s (%s):\n", pod.Name, container.Name)
			runLogs(c, ns, pod.Name, container.Name, false, since, out)
			fmt.Fprintf(out, "\n\n===============\nSTOP LOGS for %s (%s):\n", pod.Name, container.Name)
		}
	}
}

func runLogs(client clientset.Interface, namespace, name, container string, previous bool, sinceStart time.Duration, out io.Writer) error {
	req := client.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Name(name).
		Resource("pods").
		SubResource("log").
		Param("container", container).
		Param("previous", strconv.FormatBool(previous)).
		Param("since", strconv.FormatInt(int64(sinceStart.Round(time.Second).Seconds()), 10))

	readCloser, err := req.Stream()
	if err != nil {
		return err
	}

	defer readCloser.Close()
	_, err = io.Copy(out, readCloser)
	return err

}
