package doctor

import (
	"strconv"
	"strings"

	core_util "github.com/appscode/kutil/core/v1"
	"github.com/appscode/kutil/meta"
	"github.com/appscode/kutil/tools/exec"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
)

func (d *Doctor) processPod(pod core.Pod) (*APIServerConfig, error) {
	running, err := core_util.PodRunningAndReady(pod)
	if err != nil {
		return nil, err
	}
	if !running {
		return nil, errors.Errorf("pod %s is not running", pod.Name)
	}

	if len(pod.Spec.Containers) != 1 {
		return nil, errors.Errorf("pod %s has %d containers, expected 1 container", pod.Name, len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	args := map[string]string{}
	if len(container.Command) > 1 {
		if container.Command[0] == "kube-apiserver" {
			args = meta.ParseArgumentListToMap(container.Command)
		} else if strings.HasSuffix(container.Command[0], "hyperkube") && container.Command[1] == "apiserver" {
			args = meta.ParseArgumentListToMap(container.Command[1:])
		} else {
			var cmd string
			for _, c := range container.Command {
				if strings.Contains(c, "kube-apiserver") {
					cmd = c
					break
				}
			}
			if cmd == "" {
				return nil, errors.Errorf(`pod %s is using command %s, expected "kube-apiserver"`, pod.Name, container.Command[0])
			}

			fields := strings.Fields(cmd)
			for i, w := range fields {
				if strings.HasSuffix(w, "kube-apiserver") {
					args = meta.ParseArgumentListToMap(fields[i:])
					break
				}
			}
		}
	} else if len(container.Args) > 0 {
		args = meta.ParseArgumentListToMap(container.Args)
	}

	var config APIServerConfig

	config.PodName = pod.Name
	config.NodeName = pod.Spec.NodeName
	config.PodIP = pod.Status.PodIP
	config.HostIP = pod.Status.HostIP

	{
		// ref: https://github.com/golang/go/blob/e5f0c1f6c9dc382bdc6a0ec1a0d5e1fc6f833485/src/net/http/transport.go#L35
		config.ProxySettings = map[string]string{}
		for _, e := range container.Env {
			switch e.Name {
			case "no_proxy":
			case "NO_PROXY":
			case "http_proxy":
			case "HTTP_PROXY":
				config.ProxySettings[e.Name] = e.Value
			}
		}
	}

	if v, ok := args["admission-control"]; ok && v != "" {
		config.AdmissionControl = strings.Split(v, ",")
	}
	if v, ok := args["enable-admission-plugins"]; ok && v != "" {
		config.AdmissionControl = strings.Split(v, ",")
	}

	if v, ok := args["client-ca-file"]; ok && v != "" {
		data, err := exec.ExecIntoPod(d.config, &pod, "cat", v)
		if err != nil {
			return nil, err
		}
		config.ClientCAData = strings.TrimSpace(data)
	}

	if v, ok := args["tls-cert-file"]; ok && v != "" {
		data, err := exec.ExecIntoPod(d.config, &pod, "cat", v)
		if err != nil {
			return nil, err
		}
		config.TLSCertData = strings.TrimSpace(data)
	}

	if v, ok := args["requestheader-client-ca-file"]; ok && v != "" {
		data, err := exec.ExecIntoPod(d.config, &pod, "cat", v)
		if err != nil {
			return nil, err
		}
		config.RequestHeaderCAData = strings.TrimSpace(data)
	}

	config.AllowPrivileged, err = strconv.ParseBool(args["allow-privileged"])
	if err != nil {
		return nil, err
	}

	if v, ok := args["authorization-mode"]; ok && v != "" {
		config.AuthorizationMode = strings.Split(v, ",")
	}

	if v, ok := args["runtime-config"]; ok && v != "" {
		apis := strings.Split(v, ",")
		for _, api := range apis {
			parts := strings.SplitN(api, "=", 2)
			if len(parts) == 2 {
				if e, _ := strconv.ParseBool(parts[1]); e {
					config.RuntimeConfig.Enabled = append(config.RuntimeConfig.Enabled, parts[0])
				} else {
					config.RuntimeConfig.Disabled = append(config.RuntimeConfig.Disabled, parts[0])
				}
			} else {
				config.RuntimeConfig.Enabled = append(config.RuntimeConfig.Enabled, parts[0])
			}
		}
	}

	if v, ok := args["feature-gates"]; ok && v != "" {
		features := strings.Split(v, ",")
		for _, f := range features {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) == 2 {
				if e, _ := strconv.ParseBool(parts[1]); e {
					config.FeatureGates.Enabled = append(config.FeatureGates.Enabled, parts[0])
				} else {
					config.FeatureGates.Disabled = append(config.FeatureGates.Disabled, parts[0])
				}
			} else {
				config.FeatureGates.Enabled = append(config.FeatureGates.Enabled, parts[0])
			}
		}
	}
	return &config, nil
}
