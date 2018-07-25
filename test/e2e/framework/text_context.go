/*
Copyright 2016 The Kubernetes Authors.
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
	"flag"
	"os"

	"github.com/onsi/ginkgo/config"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	RecommendedConfigPathEnvVar = "MYSQLOPERATORCONFIGS"
)

type TestContextType struct {
	KubeHost    string
	KubeConfig  string
	KubeContext string

	ReportDir string

	ChartPath   string
	ChartValues string

	OperatorImage string
	HelperImage   string

	TimeoutSeconds int
}

var TestContext TestContextType

// Register flags common to all e2e test suites.
func RegisterCommonFlags() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = true

	flag.StringVar(&TestContext.KubeHost, "kubernetes-host", "http://127.0.0.1:8080", "The kubernetes host, or apiserver, to connect to")
	flag.StringVar(&TestContext.KubeConfig, "kubernetes-config", os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for kubernetes. Default value is from environment variable "+clientcmd.RecommendedConfigPathEnvVar)
	flag.StringVar(&TestContext.KubeContext, "kubernetes-context", "", "config context to use for kuberentes. If unset, will use value from 'current-context'")

	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Optional directory to store junit output in. If not specified, no junit file will be output")
	flag.StringVar(&TestContext.ChartPath, "operator-chart-path", "../../hack/charts/mysql-operator", "The chart name or path for mysql operator")
	flag.StringVar(&TestContext.ChartValues, "operator-chart-values-path", "../../test/e2e-values.yaml", "Path to a values file for mysql-operator chart.")

	flag.StringVar(&TestContext.OperatorImage, "operator-image", "quay.io/presslabs/mysql-operator:build", "Image for mysql operator.")
	flag.StringVar(&TestContext.HelperImage, "helper-image", "quay.io/presslabs/mysql-helper:build", "Image for mysql helper.")

	flag.IntVar(&TestContext.TimeoutSeconds, "pod-wait-timeout", 100, "Timeout to wait for a pod to be ready.")
}

func RegisterParseFlags() {
	RegisterCommonFlags()
	flag.Parse()
}
