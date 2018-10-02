package script

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/appscode/go/os/repos"
	"github.com/appscode/go/strings"
)

type DebianGeneric struct {
	LinuxGeneric
}

func (script *DebianGeneric) ProcessEnable(ps string) {
	// TODO(verify): unused
	os.Remove("/etc/init/" + ps + ".override")
	script.shell.Command("/usr/sbin/update-rc.d", ps, "enable").Run()
}

func (script *DebianGeneric) ProcessDisable(ps string) {
	file := "/etc/init/" + ps + ".override"
	if _, err := os.Stat(file); os.IsNotExist(err) {
		ioutil.WriteFile(file, []byte("manual"), 0600)
	}

	// update-rc.d salt-minion disable
	// service salt-minion stop
	script.shell.Command("/usr/sbin/update-rc.d", ps, "disable").Run()
	// TODO: Need a repeat loop?
	/*
		  while service salt-minion status >/dev/null; do
			echo "salt-minion found running, stopping"
			service salt-minion stop
			sleep 1
		  done
	*/
	script.shell.Command("/usr/sbin/service", ps, "stop").Run()
}

func (script *DebianGeneric) AddPPA(ppa ...string) {
	a := append([]string{"-y"}, ppa...)
	b := make([]interface{}, len(a))
	for i, s := range a {
		b[i] = s
	}
	script.shell.Command("/usr/bin/add-apt-repository", b...).Run()
	script.shell.Command("/usr/bin/apt-get", "update").Run()
}

/*
Example:
wget -O- https://repo.saltstack.com/apt/ubuntu/14.04/amd64/latest/SALTSTACK-GPG-KEY.pub | sudo apt-key add -

nano /etc/apt/sources.list.d/saltstack.list
deb http://repo.saltstack.com/apt/ubuntu/14.04/amd64/2015.8 trusty main
*/
func (script *DebianGeneric) AddRepo(r interface{}) {
	repo := r.(repos.DebianRepo)
	if repo.GPGKeyURL != "" {
		script.shell.Command("wget", "-O-", repo.GPGKeyURL).Command("apt-key", "add", "-").Run()
	}
	if repo.Deb != "" {
		script.AddLine(repo.Listing, repo.Deb)
	}
	if repo.DebSrc != "" {
		script.AddLine(repo.Listing, repo.DebSrc)
	}
	script.shell.Command("/usr/bin/apt-get", "update").Run()
}

func (script *DebianGeneric) UpdateRepos() {
	script.shell.Command("/usr/bin/apt-get", "update").Run()
}

func (script *DebianGeneric) UpgradePkgs() {
	script.shell.Command("/usr/bin/apt-get", "upgrade", "-y").Run()
}

func (script *DebianGeneric) InstallPkgDeps() {
	script.shell.Command("/usr/bin/apt-get", "install", "-f", "-y").Run()
}

func (script *DebianGeneric) InstallPkgs(pkgs ...string) {
	a := append([]string{"install", "-y"}, pkgs...)
	b := make([]interface{}, len(a))
	for i, s := range a {
		b[i] = s
	}
	script.shell.Command("/usr/bin/apt-get", b...).Run()
}

func (script *DebianGeneric) DeactivateDaemons() {
	content := `#!/bin/sh
echo "Salt shall not start." >&2
exit 101`
	ioutil.WriteFile("/usr/sbin/policy-rc.d", []byte(content), 0755)
}

func (script *DebianGeneric) ActivateDaemons() {
	file := "/usr/sbin/policy-rc.d"
	if _, err := os.Stat(file); err == nil {
		if err := os.Remove(file); err != nil {
			log.Fatal(err)
		}
	}
}

func (script *DebianGeneric) InstallSaltMaster(versions ...string) {
	v := strings.VString(repos.DefaultSaltstackVersion["debian"], versions...)
	if !script.ProcessExists("salt-master") || !script.ProcessExists("salt-minion") {
		script.AddRepo(repos.DebianRepos[v])
		script.InstallPkgs("salt-master", "salt-api", "salt-minion")
	}
}

func (script *DebianGeneric) InstallSaltMinion(versions ...string) {
	v := strings.VString(repos.DefaultSaltstackVersion["debian"], versions...)
	if !script.ProcessExists("salt-minion") {
		script.AddRepo(repos.DebianRepos[v])
		script.InstallPkgs("salt-minion")
	}
}

/*
func (script *ubuntuImpl) installSaltMinion() {
	if !script.ProcessExists("salt-minion") {
		script.InstallPkg("python-software-properties")
		script.AddRepo("ppa:saltstack/salt")
		script.InstallPkg("salt-minion")
	}
	script.ProcessRestart("salt-minion")
}

func (script *ubuntuImpl) installSaltMaster() {
	if !script.ProcessExists("salt-master") || !script.ProcessExists("salt-minion") {
		script.InstallPkg("python-software-properties")
		script.AddRepo("ppa:saltstack/salt")
		script.InstallPkg("salt-master", "salt-minion")
	}
	script.ProcessRestart("salt-minion", "salt-master")
}
*/
