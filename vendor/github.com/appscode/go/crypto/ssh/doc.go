/*
Package ssh provides a GOlang library for copying files and running commands over SSH

## Example
```go
package main

import (
	"github.com/appscode/go/crypto/ssh"
	"log"
	"os"
)

func main() {
	signer, err := ssh.MakePrivateKeySignerFromFile(os.ExpandEnv("$HOME/.ssh/id_rsa"))
	if err != nil {
		log.Fatal(err)
	}
	sout, serr, code, err := sshtools.Exec("ls -l /", "root", "<addr>:<port>", signer)
	log.Println(sout, serr, code, err)
}
```

## Acknowledgement
This library is based on code from:
 - https://github.com/kubernetes/kubernetes/tree/master/pkg/ssh
 - https://github.com/YuriyNasretdinov/GoSSHa
*/
package ssh
