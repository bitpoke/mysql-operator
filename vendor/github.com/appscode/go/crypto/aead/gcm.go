package aead

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"strings"
)

/* For background and implementation: See here: https://appscode.appscode.io/diffusion/100/ */
type Cryptor interface {
	EncryptString(plainText string, key string) (string, error)
	Encrypt(plainText []byte, key string) ([]byte, error)
	DecryptString(cipherText string, key string) (string, error)
	Decrypt(cipherText []byte, key string) ([]byte, error)
}

const (
	aesGCMKeySize   = 32
	aesGCMNonceSize = 12
)

type RealCryptor struct{}

func (c RealCryptor) EncryptString(plainText string, key string) (string, error) {
	cipherText, err := c.Encrypt([]byte(plainText), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func (c RealCryptor) Encrypt(plainText []byte, key string) ([]byte, error) {
	block, err := aes.NewCipher(fitSecret(key, aesGCMKeySize))
	if err != nil {
		return nil, err
	}

	nonce := fitSecret(key, aesGCMNonceSize)
	gcm, err := cipher.NewGCMWithNonceSize(block, len(nonce))
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plainText, nil), nil
}

func (c RealCryptor) DecryptString(cipherText string, key string) (string, error) {
	enc, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	plainText, err := c.Decrypt(enc, key)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}

func (c RealCryptor) Decrypt(cipherText []byte, key string) ([]byte, error) {
	block, err := aes.NewCipher(fitSecret(key, aesGCMKeySize))
	if err != nil {
		return nil, err
	}

	nonce := fitSecret(key, aesGCMNonceSize)
	gcm, err := cipher.NewGCMWithNonceSize(block, len(nonce))
	if err != nil {
		return nil, err
	}
	out, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func fitSecret(seed string, expectedLength int) []byte {
	return []byte(strings.Repeat(seed, expectedLength/len(seed)+1)[:expectedLength])
}

type PassThroughCryptor struct{}

func (PassThroughCryptor) EncryptString(plainText string, key string) (string, error) {
	return plainText, nil
}

func (PassThroughCryptor) Encrypt(plainText []byte, key string) ([]byte, error) {
	return plainText, nil
}

func (PassThroughCryptor) DecryptString(cipherText string, key string) (string, error) {
	return cipherText, nil
}

func (PassThroughCryptor) Decrypt(cipherText []byte, key string) ([]byte, error) {
	return cipherText, nil
}
