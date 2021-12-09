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

package framework

import (
	"strings"

	"os"
	"os/exec"

	. "github.com/onsi/gomega"
)

func setImage(p, image string) []string {
	repositoryTag := strings.SplitN(image, ":", 2)

	ret := []string{
		"--set-string", p + ".repository=" + repositoryTag[0],
	}

	if len(repositoryTag) == 2 {
		ret = append(ret, "--set-string", p+".tag="+repositoryTag[1])
	}

	return ret
}

func HelmInstallChart(release, ns string) {
	args := []string{
		"install", release, "./" + TestContext.ChartPath,
		"--namespace", ns,
		"--values", TestContext.ChartValues, "--wait",
		"--kube-context", TestContext.KubeContext,
	}
	args = append(args, setImage("image", TestContext.OperatorImage)...)
	args = append(args, setImage("sidecar57.image", TestContext.SidecarMysql57Image)...)
	args = append(args, setImage("sidecar80.image", TestContext.SidecarMysql8Image)...)
	args = append(args, setImage("orchestrator.image", TestContext.OrchestratorImage)...)

	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Expect(cmd.Run()).Should(Succeed())
}

func HelmPurgeRelease(release, ns string) {
	args := []string{
		"delete", release,
		"--namespace", ns,
		"--kube-context", TestContext.KubeContext,
	}
	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Expect(cmd.Run()).Should(Succeed())
}
