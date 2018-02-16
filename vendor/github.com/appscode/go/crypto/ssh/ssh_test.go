package ssh

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testSSHServer struct {
	Host       string
	Port       string
	Type       string
	Data       []byte
	PrivateKey []byte
	PublicKey  []byte
}

type mockSSHDialer struct {
	network string
	addr    string
	config  *ssh.ClientConfig
}

func (d *mockSSHDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	d.network = network
	d.addr = addr
	d.config = config
	return nil, fmt.Errorf("mock error from Dial")
}

type mockSigner struct {
}

func (s *mockSigner) PublicKey() ssh.PublicKey {
	panic("mockSigner.PublicKey not implemented")
}

func (s *mockSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	panic("mockSigner.Sign not implemented")
}

func TestSSHUser(t *testing.T) {
	signer := &mockSigner{}

	table := []struct {
		title      string
		user       string
		host       string
		signer     ssh.Signer
		command    string
		expectUser string
	}{
		{
			title:      "all values provided",
			user:       "testuser",
			host:       "testhost",
			signer:     signer,
			command:    "uptime",
			expectUser: "testuser",
		},
		{
			title:      "empty user defaults to GetEnv(USER)",
			user:       "",
			host:       "testhost",
			signer:     signer,
			command:    "uptime",
			expectUser: os.Getenv("USER"),
		},
	}

	for _, item := range table {
		dialer := &mockSSHDialer{}

		_, err := newClient(dialer, item.user, item.host, item.signer, false)
		if err == nil {
			t.Errorf("expected error (as mock returns error); did not get one")
		}
		errString := err.Error()
		if !strings.HasPrefix(errString, fmt.Sprintf("error getting SSH client to %s@%s:", item.expectUser, item.host)) {
			t.Errorf("unexpected error: %v", errString)
		}

		if dialer.network != "tcp" {
			t.Errorf("unexpected network: %v", dialer.network)
		}

		if dialer.config.User != item.expectUser {
			t.Errorf("unexpected user: %v", dialer.config.User)
		}
		if len(dialer.config.Auth) != 1 {
			t.Errorf("unexpected auth: %v", dialer.config.Auth)
		}
		// (No way to test Auth - nothing exported?)

	}

}

type slowDialer struct {
	delay time.Duration
	err   error
}

func (s *slowDialer) Dial(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	time.Sleep(s.delay)
	if s.err != nil {
		return nil, s.err
	}
	return &ssh.Client{}, nil
}

func TestTimeoutDialer(t *testing.T) {
	testCases := []struct {
		delay             time.Duration
		timeout           time.Duration
		err               error
		expectedErrString string
	}{
		// delay > timeout should cause ssh.Dial to timeout.
		{1 * time.Second, 0, nil, "timed out dialing"},
		// delay < timeout should return the result of the call to the dialer.
		{0, 1 * time.Second, nil, ""},
		{0, 1 * time.Second, fmt.Errorf("test dial error"), "test dial error"},
	}
	for _, tc := range testCases {
		dialer := &timeoutDialer{&slowDialer{tc.delay, tc.err}, tc.timeout}
		_, err := dialer.Dial("tcp", "addr:port", &ssh.ClientConfig{})
		if len(tc.expectedErrString) == 0 && err != nil ||
			!strings.Contains(fmt.Sprint(err), tc.expectedErrString) {
			t.Errorf("Expected error to contain %q; got %v", tc.expectedErrString, err)
		}
	}
}
