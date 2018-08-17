/*
Copyright 2018 Pressinfra SRL

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
	"os/exec"

	. "github.com/onsi/gomega"

	"github.com/presslabs/mysql-operator/test/e2e/framework"
)

func HelmInstallChart(release string) {
	args := []string{
		"install", "./" + framework.TestContext.ChartPath,
		"--namespace", operatorNamespace,
		"--name", release, "--values", framework.TestContext.ChartValues, "--wait",
		"--kube-context", framework.TestContext.KubeContext,
		"--set", fmt.Sprintf("image=%s", framework.TestContext.OperatorImage),
		"--set", fmt.Sprintf("helperImage=%s", framework.TestContext.HelperImage),
	}

	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Expect(cmd.Run()).Should(Succeed())
}

func HelmPurgeRelease(release string) {
	args := []string{
		"delete", "--purge", release,
		"--kube-context", framework.TestContext.KubeContext,
	}
	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Expect(cmd.Run()).Should(Succeed())
}
