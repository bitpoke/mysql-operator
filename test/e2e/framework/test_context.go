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

	"github.com/bitpoke/mysql-operator/pkg/version"

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

	OperatorImage       string
	SidecarMysql57Image string
	SidecarMysql8Image  string
	OrchestratorImage   string

	TimeoutSeconds    int
	DumpLogsOnFailure bool
}

var TestContext TestContextType

// Register flags common to all e2e test suites.
func RegisterCommonFlags() {
	commit := version.GetInfo().GitCommit

	flag.StringVar(&TestContext.KubeHost, "kubernetes-host", "", "The kubernetes host, or apiserver, to connect to")
	flag.StringVar(&TestContext.KubeConfig, "kubernetes-config", os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to config containing embedded authinfo for kubernetes. Default value is from environment variable "+clientcmd.RecommendedConfigPathEnvVar)
	flag.StringVar(&TestContext.KubeContext, "kubernetes-context", "", "config context to use for kuberentes. If unset, will use value from 'current-context'")

	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Optional directory to store junit and pod logs output in. If not specified, no junit or logs files will be output")
	flag.StringVar(&TestContext.ChartPath, "operator-chart-path", "../../deploy/charts/mysql-operator", "The chart name or path for mysql operator")
	flag.StringVar(&TestContext.ChartValues, "operator-chart-values-path", "../../test/e2e-values.yaml", "Path to a values file for mysql-operator chart.")

	flag.StringVar(&TestContext.OperatorImage, "operator-image", "eu.gcr.io/bitpoke-mysql-operator-testing/mysql-operator:"+commit, "Image for mysql operator.")
	flag.StringVar(&TestContext.SidecarMysql57Image, "sidecar-mysql57-image", "eu.gcr.io/bitpoke-mysql-operator-testing/mysql-operator-sidecar-5.7:"+commit, "Image for mysql helper.")
	flag.StringVar(&TestContext.SidecarMysql8Image, "sidecar-mysql8-image", "eu.gcr.io/bitpoke-mysql-operator-testing/mysql-operator-sidecar-8.0:"+commit, "Image for mysql helper.")
	flag.StringVar(&TestContext.OrchestratorImage, "orchestrator-image", "eu.gcr.io/bitpoke-mysql-operator-testing/mysql-operator-orchestrator:"+commit, "Image for mysql orchestrator.")

	flag.IntVar(&TestContext.TimeoutSeconds, "pod-wait-timeout", 100, "Timeout to wait for a pod to be ready.")
	flag.BoolVar(&TestContext.DumpLogsOnFailure, "dump-logs-on-failure", true, "Dump pods logs when a test fails.")
}

func RegisterParseFlags() {
	RegisterCommonFlags()
	flag.Parse()
}
