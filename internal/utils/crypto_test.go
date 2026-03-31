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

package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func TestParseRSAPrivateKey_PKCS1(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	parsed, err := ParseRSAPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestParseRSAPrivateKey_PKCS8(t *testing.T) {
	t.Parallel()
	key, _ := testutil.GenerateRSAKey(t)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	parsed, err := ParseRSAPrivateKey(string(pemBytes))
	require.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestParseRSAPrivateKey_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{name: "not_pem", data: "not a pem key"},
		{name: "empty", data: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseRSAPrivateKey(tt.data)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidPrivateKey)
		})
	}
}

func TestParseRSAPrivateKey_NonRSA_PKCS8(t *testing.T) {
	t.Parallel()

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(ecKey)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err = ParseRSAPrivateKey(string(pemBytes))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotRSAKey)
}

func TestHashToken(t *testing.T) {
	t.Parallel()
	h := HashString("ghs_abc123")
	assert.True(t, strings.HasPrefix(h, "sha256:"))
	assert.Len(t, h, len("sha256:")+64)
	assert.Equal(t, h, HashString("ghs_abc123"))
	assert.NotEqual(t, h, HashString("ghs_other"))
}

func TestParseRSAPrivateKey_CorruptedPEMBody(t *testing.T) {
	t.Parallel()

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("this is not valid DER data"),
	})

	_, err := ParseRSAPrivateKey(string(pemBytes))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPrivateKey)
}
