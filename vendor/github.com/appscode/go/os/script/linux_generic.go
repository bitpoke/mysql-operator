package script

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	sh "github.com/codeskyblue/go-sh"
)

// Linux version must use Systemd as init process
type LinuxGeneric struct {
	appDir     string
	workingDir string
	shell      *sh.Session
}

func (script *LinuxGeneric) Init(workingDir string) {
	// appDir is used to download various applications.
	script.appDir = "/tmp/app"
	script.Mkdir(script.appDir)

	script.workingDir = workingDir

	// create shell session running from `workingDir`
	script.shell = sh.NewSession()
	script.shell.SetDir(script.workingDir)
	script.shell.ShowCMD = true
}

func (script *LinuxGeneric) AppDir() string {
	return script.appDir
}

func (script *LinuxGeneric) WorkingDir() string {
	return script.workingDir
}

func (script *LinuxGeneric) Raw() interface{} {
	return script.shell
}

func (script *LinuxGeneric) Mkdir(path string) {
	if err := os.MkdirAll(path, 0777); err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) Chmod(name string, mode os.FileMode) {
	if err := os.Chmod(name, mode); err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) Symlink(oldname, newname string) {
	if err := script.shell.Command("ln", "-sf", oldname, newname).Run(); err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) ChownRecurse(name string, u string) {
	if err := script.shell.Command("chown", "-R", u, name).Run(); err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) ChgrpRecurse(name string, g string) {
	if err := script.shell.Command("chgrp", "-R", g, name).Run(); err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) CmdOut(def string, cmd string, args ...interface{}) string {
	out, _ := script.shell.Command(cmd, args...).Output()
	result := strings.TrimSpace(string(out))
	log.Println("RESULT=", result)
	if len(result) > 0 {
		return result
	}
	return def
}

func (script *LinuxGeneric) Run(cmd string, args ...interface{}) error {
	return script.shell.Command(cmd, args...).Run()
}

func (script *LinuxGeneric) SetEnv(key, value string) {
	script.shell.SetEnv(key, value)
}

func (script *LinuxGeneric) UserExists(u string) bool {
	// https://github.com/docker/docker/issues/1023
	// DO NOT use user.Lookup(u), because it needs cgo
	return script.shell.Command("id", "-u", u).Run() == nil
}

func (script *LinuxGeneric) AddLine(file string, line string) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Avoids uncommenting existing line
		if strings.TrimSpace(scanner.Text()) == line {
			return
		}
	}
	_, err = f.WriteString(line + "\n")
	if err != nil {
		log.Fatal(err)
	}
}

func (script *LinuxGeneric) UncommentLine(file string, regex string) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	content := ""
	for _, line := range strings.Split(string(data), "\n") {
		if match, _ := regexp.MatchString(regex, line); !match {
			content += line
			content += "\n"
		}
	}
	ioutil.WriteFile(file, []byte(content), 0600)
}

func (script *LinuxGeneric) CheckPathExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic(fmt.Errorf("%v not found", path))
	}
}

func (script *LinuxGeneric) ReadAsBase64(path string) string {
	bytes, _ := ioutil.ReadFile(path)
	return base64.StdEncoding.EncodeToString([]byte(string(bytes)))
}

func (script *LinuxGeneric) WriteBase64String(filename string, data string) {
	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile(filename, bytes, 0600)
}

func (script *LinuxGeneric) WriteString(filename string, data string) {
	ioutil.WriteFile(filename, []byte(data), 0600)
}

func (script *LinuxGeneric) WriteBytes(filename string, data []byte) {
	ioutil.WriteFile(filename, data, 0600)
}

func (script *LinuxGeneric) ProcessExists(ps string) bool {
	out, err := script.shell.Command("which", ps).Output()
	return err == nil && string(out) != ""
}

func (script *LinuxGeneric) ProcessStart(ps ...string) {
	a := append([]string{"start"}, ps...)
	b := make([]interface{}, len(a))
	for i, s := range a {
		b[i] = s
	}
	script.shell.Command("/bin/systemctl", b...).Run()
}

func (script *LinuxGeneric) ProcessRestart(ps ...string) {
	a := append([]string{"restart"}, ps...)
	b := make([]interface{}, len(a))
	for i, s := range a {
		b[i] = s
	}
	script.shell.Command("/bin/systemctl", b...).Run()
}

func (script *LinuxGeneric) DeactivateDaemons() {}

func (script *LinuxGeneric) ActivateDaemons() {}
