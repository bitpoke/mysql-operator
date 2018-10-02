package script

import "os"

type Script interface {
	Init(workingDir string)

	AppDir() string
	WorkingDir() string
	Raw() interface{}
	CmdOut(def string, cmd string, args ...interface{}) string
	Run(cmd string, args ...interface{}) error
	SetEnv(key, value string)

	Mkdir(path string)
	Chmod(name string, mode os.FileMode)
	Symlink(oldname, newname string)
	ChownRecurse(name string, u string)
	ChgrpRecurse(name string, g string)

	UserExists(u string) bool

	CheckPathExists(path string)
	ReadAsBase64(path string) string
	WriteBase64String(filename string, data string)
	WriteString(filename string, data string)
	WriteBytes(filename string, data []byte)

	AddLine(file string, line string)
	UncommentLine(file string, regex string)

	ProcessExists(ps string) bool
	ProcessEnable(ps string)
	ProcessStart(ps ...string)
	ProcessRestart(ps ...string)
	ProcessDisable(ps string)

	AddRepo(r interface{})
	UpdateRepos()
	UpgradePkgs()
	InstallPkgDeps()

	InstallPkgs(pkgs ...string)
	DeactivateDaemons()
	ActivateDaemons()

	InstallSaltMaster(versions ...string)
	InstallSaltMinion(versions ...string)
}
