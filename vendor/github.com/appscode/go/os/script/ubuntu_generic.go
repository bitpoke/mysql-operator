package script

import (
	"github.com/appscode/go/os/repos"
	"github.com/appscode/go/strings"
)

type UbuntuGeneric struct {
	DebianGeneric
}

func (script *UbuntuGeneric) ProcessEnable(ps string) {
	script.shell.Command("/bin/systemctl", "enable", ps).Run()
}

func (script *UbuntuGeneric) ProcessDisable(ps string) {
	script.shell.Command("/bin/systemctl", "stop").Run()
	script.shell.Command("/bin/systemctl", "disable").Run()
}

func (script *UbuntuGeneric) InstallSaltMaster(versions ...string) {
	v := strings.VString(repos.DefaultSaltstackVersion["ubuntu"], versions...)
	script.DebianGeneric.InstallSaltMaster(v)
}

func (script *UbuntuGeneric) InstallSaltMinion(versions ...string) {
	v := strings.VString(repos.DefaultSaltstackVersion["ubuntu"], versions...)
	script.DebianGeneric.InstallSaltMinion(v)
}
