/*
Copyright 2018 Pressinfra SRL.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package rand provide functions for securely generating random strings. It
// uses crypto/rand to securely generate random sequences of characters.
// It is adapted from https://gist.github.com/denisbrodbeck/635a644089868a51eccd6ae22b2eb800
// to support multiple character sets.
package rand

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
)

const (
	lowerLetters  = "abcdefghijklmnopqrstuvwxyz"
	upperLetters  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letters       = lowerLetters + upperLetters
	digits        = "0123456789"
	alphanumerics = letters + digits
	ascii         = alphanumerics + "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

// NewStringGenerator generate a cryptographically secure random sequence
// generator from given characters.
func NewStringGenerator(characters string) func(int) (string, error) {
	return func(length int) (string, error) {
		result := ""
		for {
			if len(result) >= length {
				return result, nil
			}
			num, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
			if err != nil {
				return "", err
			}
			n := num.Int64()
			result += string(characters[n])
		}
	}
}

var alphaNumericStringGenerator = NewStringGenerator(alphanumerics)

// AlphaNumericString returns a cryptographically secure random sequence of
// alphanumeric characters.
func AlphaNumericString(length int) (string, error) {
	return alphaNumericStringGenerator(length)
}

var lowerAlphaNumericStringGenerator = NewStringGenerator(lowerLetters + digits)

// LowerAlphaNumericString returns a cryptographically secure random sequence of
// lower alphanumeric characters.
func LowerAlphaNumericString(length int) (string, error) {
	return lowerAlphaNumericStringGenerator(length)
}

var asciiStringGenerator = NewStringGenerator(ascii)

// ASCIIString returns a cryptographically secure random sequence of
// printable ASCII characters, excluding space.
func ASCIIString(length int) (string, error) {
	return asciiStringGenerator(length)
}

func init() {
	assertAvailablePRNG()
}

func assertAvailablePRNG() {
	// Assert that a cryptographically secure PRNG is available.
	// Panic otherwise.
	buf := make([]byte, 1)

	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand is unavailable: Read() failed with %#v", err))
	}
}
