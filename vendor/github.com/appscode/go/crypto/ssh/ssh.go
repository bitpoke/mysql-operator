package ssh

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/appscode/go/runtime"
	"github.com/appscode/go/wait"
	"golang.org/x/crypto/ssh"
)

// Interface to allow mocking of ssh.Dial, for testing SSH
type sshDialer interface {
	Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

// Real implementation of sshDialer
type realSSHDialer struct{}

var _ sshDialer = &realSSHDialer{}

func (d *realSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return ssh.Dial(network, addr, config)
}

// timeoutDialer wraps an sshDialer with a timeout around Dial(). The golang
// ssh library can hang indefinitely inside the Dial() call (see issue #23835).
// Wrapping all Dial() calls with a conservative timeout provides safety against
// getting stuck on that.
type timeoutDialer struct {
	dialer  sshDialer
	timeout time.Duration
}

// 150 seconds is longer than the underlying default TCP backoff delay (127
// seconds). This timeout is only intended to catch otherwise uncaught hangs.
const sshDialTimeout = 150 * time.Second

const chunkSize = 65536 // chunk size in bytes for scp

var realTimeoutDialer sshDialer = &timeoutDialer{&realSSHDialer{}, sshDialTimeout}

func (d *timeoutDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	var client *ssh.Client
	errCh := make(chan error, 1)
	go func() {
		defer runtime.HandleCrash()
		var err error
		client, err = d.dialer.Dial(network, addr, config)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return client, err
	case <-time.After(d.timeout):
		return nil, fmt.Errorf("timed out dialing %s:%s", network, addr)
	}
}

type Client struct {
	*ssh.Client
	host string
	user string
}

// Internal implementation of runSSHCommand, for testing
func NewClient(user, host string, signer ssh.Signer) (*Client, error) {
	return newClient(realTimeoutDialer, user, host, signer, true)
}

// Internal implementation of runSSHCommand, for testing
func newClient(dialer sshDialer, user, host string, signer ssh.Signer, retry bool) (*Client, error) {
	if user == "" {
		user = os.Getenv("USER")
	}
	// Setup the config, dial the server, and open a session.
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	client, err := dialer.Dial("tcp", host, config)
	if err != nil && retry {
		err = wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
			fmt.Printf("error dialing %s@%s: '%v', retrying\n", user, host, err)
			if client, err = dialer.Dial("tcp", host, config); err != nil {
				return false, nil
			}
			return true, nil
		})
	}
	if err != nil {
		return nil, fmt.Errorf("error getting SSH client to %s@%s: '%v'", user, host, err)
	}
	return &Client{
		Client: client,
		user:   user,
		host:   host,
	}, nil
}

// Internal implementation of runSSHCommand, for testing
func (c *Client) Exec(cmd string) (string, string, int, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", "", 0, fmt.Errorf("error creating session to %s@%s: '%v'", c.user, c.host, err)
	}
	defer session.Close()

	// Run the command.
	code := 0
	var bout, berr bytes.Buffer
	session.Stdout, session.Stderr = &bout, &berr
	if err = session.Run(cmd); err != nil {
		// Check whether the command failed to run or didn't complete.
		if exiterr, ok := err.(*ssh.ExitError); ok {
			// If we got an ExitError and the exit code is nonzero, we'll
			// consider the SSH itself successful (just that the command run
			// errored on the host).
			if code = exiterr.ExitStatus(); code != 0 {
				err = nil
			}
		} else {
			// Some other kind of error happened (e.g. an IOError); consider the
			// SSH unsuccessful.
			err = fmt.Errorf("failed running `%s` on %s@%s: '%v'", cmd, c.user, c.host, err)
		}
	}
	return bout.String(), berr.String(), code, err
}

// Internal implementation of runSSHCommand, for testing
func (c *Client) SCP(dst string, contents []byte) (string, string, int, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", "", 0, fmt.Errorf("error creating session to %s@%s: '%v'", c.user, c.host, err)
	}
	defer session.Close()

	// Run the command.
	cmd := "cat >'" + strings.Replace(dst, "'", "'\\''", -1) + "'"
	bin, err := session.StdinPipe()
	if err != nil {
		return "", "", 0, err
	}
	code := 0
	var bout, berr bytes.Buffer
	session.Stdout, session.Stderr = &bout, &berr

	err = session.Start(cmd)
	if err != nil {
		return "", "", 0, err
	}

	for start, maxEnd := 0, len(contents); start < maxEnd; start += chunkSize {
		end := start + chunkSize
		if end > maxEnd {
			end = maxEnd
		}
		_, err = bin.Write(contents[start:end])
		if err != nil {
			return "", "", 0, err
		}
	}
	err = bin.Close()
	if err != nil {
		return "", "", 0, err
	}

	if err = session.Wait(); err != nil {
		// Check whether the command failed to run or didn't complete.
		if exiterr, ok := err.(*ssh.ExitError); ok {
			// If we got an ExitError and the exit code is nonzero, we'll
			// consider the SSH itself successful (just that the command run
			// errored on the host).
			if code = exiterr.ExitStatus(); code != 0 {
				err = nil
			}
		} else {
			// Some other kind of error happened (e.g. an IOError); consider the
			// SSH unsuccessful.
			err = fmt.Errorf("failed running `%s` on %s@%s: '%v'", cmd, c.user, c.host, err)
		}
	}
	return bout.String(), berr.String(), code, err
}

// RunSSHCommand returns the stdout, stderr, and exit code from running cmd on
// host as specific user, along with any SSH-level error.
// If user=="", it will default (like SSH) to os.Getenv("USER")
func Exec(cmd, user, host string, signer ssh.Signer) (string, string, int, error) {
	client, err := NewClient(user, host, signer)
	if err != nil {
		return "", "", 0, err
	}
	return client.Exec(cmd)
}

// UploadFile returns the stdout, stderr, and exit code from creating a destination file on
// host as specific user, along with any SSH-level error.
// If user=="", it will default (like SSH) to os.Getenv("USER")
func SCP(dst string, contents []byte, user, host string, signer ssh.Signer) (string, string, int, error) {
	client, err := NewClient(user, host, signer)
	if err != nil {
		return "", "", 0, err
	}
	return client.SCP(dst, contents)
}
