// Copyright 2026 Thomson Reuters
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package testutil provides shared test helpers used across multiple packages.
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const rsaKeyBits = 2048

// GenerateRSAKey creates a 2048-bit RSA private key and returns both the key
// object and its PKCS#1 PEM-encoded string. It fails the test on error.
func GenerateRSAKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, string(pemBytes)
}

// GenerateRSAKeyObject creates a 2048-bit RSA private key and returns it.
// It fails the test on error.
func GenerateRSAKeyObject(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, _ := GenerateRSAKey(t)
	return key
}

// WriteKeyFile writes an RSA private key to a temporary PEM file and returns
// its path. The file is removed automatically when the test finishes.
func WriteKeyFile(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "private-key.pem")
	require.NoError(t, os.WriteFile(path, pemBytes, 0600))
	return path
}
