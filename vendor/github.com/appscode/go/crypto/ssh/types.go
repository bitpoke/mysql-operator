package ssh

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/appscode/go/errors"
	"golang.org/x/crypto/ssh"
)

const RSABitSize = 2048

type SSHKey struct {
	PublicKey          []byte `json:"publicKey,omitempty"`
	PrivateKey         []byte `json:"privateKey,omitempty"`
	AwsFingerprint     string `json:"awsFingerprint,omitempty"`
	OpensshFingerprint string `json:"opensshFingerprint,omitempty"`
}

// Source: https://github.com/flynn/flynn/blob/master/pkg/sshkeygen/sshkeygen.go
// This generates a single RSA 2048-bit SSH key
//AWS Key pair:
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html#verify-key-pair-fingerprints
// https://forums.aws.amazon.com/thread.jspa?messageID=386670&tstart=0
//
// From PUB key: ssh-keygen -f ~/.ssh/id_rsa.pub -e -m PKCS8 | openssl pkey -pubin -outform DER | openssl md5 -c
// From PRIV key: openssl rsa -in ~/.ssh/id_rsa -pubout -outform DER | openssl md5 -c
//
func NewSSHKeyPair() (*SSHKey, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, RSABitSize)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}

	rsaPubKey, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}

	k := &SSHKey{}
	k.PublicKey = bytes.TrimSpace(ssh.MarshalAuthorizedKey(rsaPubKey))
	k.PrivateKey = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	k.OpensshFingerprint = sshFingerprint(rsaPubKey.Marshal())

	der, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	k.AwsFingerprint = sshFingerprint(der)
	return k, nil
}

// https://github.com/dragon3/crypto/commit/c0e91eed7513e4213ff337635bd13d3fd0c714d0
// Replace with GO 1.6 when released
func sshFingerprint(data []byte) string {
	md5sum := md5.Sum(data)
	return rfc4716hex(md5sum[:])
}

func rfc4716hex(data []byte) string {
	var fingerprint string
	for i := 0; i < len(data); i++ {
		fingerprint = fmt.Sprintf("%v%0.2x", fingerprint, data[i])
		if i != len(data)-1 {
			fingerprint = fingerprint + ":"
		}
	}
	return fingerprint
}

func ParseSSHKeyPair(pub, priv string) (*SSHKey, error) {
	pub = strings.TrimSpace(pub)

	k := &SSHKey{}
	k.PublicKey = bytes.TrimSpace([]byte(pub))
	block, _ := pem.Decode([]byte(priv))
	k.PrivateKey = pem.EncodeToMemory(block)

	pubWireb64 := pub[strings.Index(pub, " ")+1:]
	pubBytes, err := base64.StdEncoding.DecodeString(pubWireb64)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	k.OpensshFingerprint = sshFingerprint(pubBytes)

	// Convert from ssh.rsaPublicKey -> rsa.PublicKey
	sshPK, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub))
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	b, err := json.Marshal(sshPK)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	var rsaPK rsa.PublicKey
	err = json.Unmarshal(b, &rsaPK)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	der, err := x509.MarshalPKIXPublicKey(&rsaPK)
	if err != nil {
		return nil, errors.FromErr(err).Err()
	}
	k.AwsFingerprint = sshFingerprint(der)

	return k, nil
}
