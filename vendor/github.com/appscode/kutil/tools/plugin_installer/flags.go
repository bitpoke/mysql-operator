package plugin_installer

import (
	"flag"
	"os"
	"strings"

	"github.com/spf13/pflag"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
)

func BindGlobalFlags(flags *pflag.FlagSet, plugin bool) clientcmd.ClientConfig {
	flags.AddGoFlagSet(flag.CommandLine)
	// Normalize all flags that are coming from other packages or pre-configurations
	// a.k.a. change all "_" to "-". e.g. glog package
	flags.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	clientConfig := util.DefaultClientConfig(flags)
	if plugin {
		LoadFromEnv(flags, "kubeconfig", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagClusterName, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagAuthInfoName, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagContext, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagNamespace, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagAPIServer, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagInsecure, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagCertFile, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagKeyFile, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagCAFile, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagBearerToken, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagImpersonate, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagImpersonateGroup, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagUsername, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagPassword, "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, clientcmd.FlagTimeout, "KUBECTL_PLUGINS_GLOBAL_FLAG_")

		LoadFromEnv(flags, "alsologtostderr", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "log-backtrace-at", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "log-dir", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "logtostderr", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "stderrthreshold", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "v", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
		LoadFromEnv(flags, "vmodule", "KUBECTL_PLUGINS_GLOBAL_FLAG_")
	}
	return clientConfig
}

func LoadFromEnv(flags *pflag.FlagSet, name, prefix string) {
	v, found := os.LookupEnv(plugins.FlagToEnvName(name, prefix))
	if !found {
		return
	}
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		v = v[1 : len(v)-1]
		if v == "" {
			return
		}
	}
	flags.Set(name, v)
}

func LoadFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(f *pflag.Flag) {
		LoadFromEnv(flags, f.Name, "KUBECTL_PLUGINS_LOCAL_FLAG_")
	})
}
