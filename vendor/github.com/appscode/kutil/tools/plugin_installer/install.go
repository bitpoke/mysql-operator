package plugin_installer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	ioutilz "github.com/appscode/go/ioutil"
	"github.com/appscode/go/log"
	"github.com/ghodss/yaml"
	"github.com/kardianos/osext"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
)

func NewCmdInstall(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install",
		Short:             "Install as kubectl plugin",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			dir := filepath.Join(homedir.HomeDir(), ".kube", "plugins", rootCmd.Name())
			os.MkdirAll(dir, 0755)

			p, err := osext.Executable()
			if err != nil {
				log.Fatal(err)
			}
			p = filepath.Clean(p)
			ioutilz.CopyFile(filepath.Join(dir, filepath.Base(p)), p)

			var traverse func(cmd *cobra.Command, p *plugins.Plugin)
			traverse = func(cmd *cobra.Command, p *plugins.Plugin) {
				p.Name = cmd.Name()
				p.ShortDesc = cmd.Short
				p.LongDesc = cmd.Long
				p.Example = cmd.Example

				if len(cmd.Commands()) == 0 {
					p.Command = "./" + strings.TrimSpace(cmd.CommandPath())
					fn := func(flag *pflag.Flag) {
						if flag.Hidden {
							return
						}
						p.Flags = append(p.Flags, plugins.Flag{
							Name:      flag.Name,
							Shorthand: flag.Shorthand,
							Desc:      flag.Usage,
							DefValue:  flag.DefValue,
						})
					}
					cmd.NonInheritedFlags().VisitAll(fn)
					cmd.InheritedFlags().VisitAll(fn)
				}

				for _, cc := range cmd.Commands() {
					cp := &plugins.Plugin{}
					traverse(cc, cp)
					p.Tree = append(p.Tree, cp)
				}
			}

			plugin := &plugins.Plugin{}
			traverse(rootCmd, plugin)

			data, err := yaml.Marshal(plugin)
			if err != nil {
				log.Fatal(err)
			}
			ioutil.WriteFile(filepath.Join(dir, "plugin.yaml"), data, 0755)
		},
	}
	return cmd
}
