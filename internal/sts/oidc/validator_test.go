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

package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/testutil"
)

type mockServer struct {
	*httptest.Server
	key   *rsa.PrivateKey
	keyID string
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()

	key, _ := testutil.GenerateRSAKey(t)

	const kid = "test-key-1"
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   server.URL,
			"jwks_uri": server.URL + "/jwks",
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: kid, Algorithm: string(jose.RS256), Use: "sig"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})

	t.Cleanup(server.Close)
	return &mockServer{Server: server, key: key, keyID: kid}
}

func (m *mockServer) sign(t *testing.T, claims map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), m.keyID),
	)
	require.NoError(t, err)

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	jws, err := signer.Sign(payload)
	require.NoError(t, err)

	compact, err := jws.CompactSerialize()
	require.NoError(t, err)
	return compact
}

func (m *mockServer) validClaims(aud string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":          m.URL,
		"sub":          "repo:example-org/example-repo:ref:refs/heads/main",
		"aud":          aud,
		"exp":          josejwt.NewNumericDate(now.Add(time.Hour)),
		"iat":          josejwt.NewNumericDate(now.Add(-time.Minute)),
		"repository":   "example-org/example-repo",
		"ref":          "refs/heads/main",
		"environment":  "production",
		"workflow_ref": "example-org/example-repo/.github/workflows/deploy.yml@refs/heads/main",
		"actor":        "dependabot[bot]",
	}
}

func TestNewValidator(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		o, err := NewValidator("gate", []string{"https://oidc-provider-1.example.com", "https://oidc-provider-2.example.com"})
		require.NoError(t, err)
		assert.Equal(t, "gate", o.audience)
		assert.Len(t, o.issuers, 2)
	})

	t.Run("empty_audience", func(t *testing.T) {
		t.Parallel()
		_, err := NewValidator("", []string{"https://oidc.example.com"})
		require.ErrorIs(t, err, ErrEmptyAudience)
	})

	t.Run("nil_issuers", func(t *testing.T) {
		t.Parallel()
		_, err := NewValidator("gate", nil)
		require.ErrorIs(t, err, ErrEmptyIssuers)
	})

	t.Run("empty_issuers", func(t *testing.T) {
		t.Parallel()
		_, err := NewValidator("gate", []string{})
		require.ErrorIs(t, err, ErrEmptyIssuers)
	})
}

func TestValidate_Success(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, err := NewValidator("gate", []string{server.URL})
	require.NoError(t, err)

	claims, err := o.Validate(t.Context(), server.sign(t, server.validClaims("gate")))
	require.NoError(t, err)

	assert.Equal(t, server.URL, claims.Issuer)
	assert.Equal(t, "repo:example-org/example-repo:ref:refs/heads/main", claims.Subject)
	assert.Contains(t, claims.Audience, "gate")
	assert.False(t, claims.ExpiresAt.IsZero())
	assert.False(t, claims.IssuedAt.IsZero())
}

func TestValidate_CustomClaims(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	claims, err := o.Validate(t.Context(), server.sign(t, server.validClaims("gate")))
	require.NoError(t, err)

	assert.Equal(t, "example-org/example-repo", claims.Custom["repository"])
	assert.Equal(t, "refs/heads/main", claims.Custom["ref"])
	assert.Equal(t, "production", claims.Custom["environment"])
	assert.Equal(t, "example-org/example-repo/.github/workflows/deploy.yml@refs/heads/main", claims.Custom["workflow_ref"])
	assert.Equal(t, "dependabot[bot]", claims.Custom["actor"])

	for _, key := range []string{"iss", "sub", "aud", "exp", "iat"} {
		assert.NotContains(t, claims.Custom, key)
	}
}

func TestValidate_EmptyToken(t *testing.T) {
	t.Parallel()

	o, _ := NewValidator("gate", []string{"https://oidc.example.com"})
	_, err := o.Validate(t.Context(), "")
	require.ErrorIs(t, err, ErrEmptyToken)
}

func TestValidate_MalformedToken(t *testing.T) {
	t.Parallel()

	o, _ := NewValidator("gate", []string{"https://oidc.example.com"})
	_, err := o.Validate(t.Context(), "garbage-not-a-jwt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing token")
}

func TestValidate_MissingIssuer(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	claims := server.validClaims("gate")
	delete(claims, "iss")

	_, err := o.Validate(t.Context(), server.sign(t, claims))
	require.ErrorIs(t, err, ErrMissingIssuer)
}

func TestValidate_IssuerDenied(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{"https://oidc.example.org"})

	_, err := o.Validate(t.Context(), server.sign(t, server.validClaims("gate")))
	require.ErrorIs(t, err, ErrIssuerDenied)
	assert.Contains(t, err.Error(), server.URL)
}

func TestValidate_WrongAudience(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	_, err := o.Validate(t.Context(), server.sign(t, server.validClaims("wrong-audience")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verifying token")
}

func TestValidate_ExpiredToken(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	claims := server.validClaims("gate")
	claims["exp"] = josejwt.NewNumericDate(time.Now().Add(-time.Hour))

	_, err := o.Validate(t.Context(), server.sign(t, claims))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exp")
}

func TestValidate_DiscoveryFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	o, _ := NewValidator("gate", []string{server.URL})

	key, _ := testutil.GenerateRSAKey(t)
	signer, _ := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), "k1"),
	)
	now := time.Now()
	payload, _ := json.Marshal(map[string]any{
		"iss": server.URL, "sub": "test", "aud": "gate",
		"exp": josejwt.NewNumericDate(now.Add(time.Hour)),
		"iat": josejwt.NewNumericDate(now),
	})
	jws, _ := signer.Sign(payload)
	token, _ := jws.CompactSerialize()

	_, err := o.Validate(t.Context(), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovering issuer")
}

func TestValidate_WrongSigningKey(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	wrongKey, _ := testutil.GenerateRSAKey(t)
	signer, _ := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: wrongKey},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), server.keyID),
	)
	now := time.Now()
	payload, _ := json.Marshal(map[string]any{
		"iss": server.URL, "sub": "test", "aud": "gate",
		"exp": josejwt.NewNumericDate(now.Add(time.Hour)),
		"iat": josejwt.NewNumericDate(now),
	})
	jws, _ := signer.Sign(payload)
	token, _ := jws.CompactSerialize()

	_, err := o.Validate(t.Context(), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verifying token")
}

func TestValidate_ProviderCaching(t *testing.T) {
	t.Parallel()

	discoveryHits := 0
	key, _ := testutil.GenerateRSAKey(t)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		discoveryHits++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   server.URL,
			"jwks_uri": server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "k1", Algorithm: string(jose.RS256), Use: "sig"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})

	o, _ := NewValidator("gate", []string{server.URL})

	sign := func() string {
		signer, _ := jose.NewSigner(
			jose.SigningKey{Algorithm: jose.RS256, Key: key},
			(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), "k1"),
		)
		now := time.Now()
		payload, _ := json.Marshal(map[string]any{
			"iss": server.URL, "sub": "repo:example-org/example-repo:ref:refs/heads/main", "aud": "gate",
			"exp": josejwt.NewNumericDate(now.Add(time.Hour)),
			"iat": josejwt.NewNumericDate(now),
		})
		jws, _ := signer.Sign(payload)
		compact, _ := jws.CompactSerialize()
		return compact
	}

	_, err := o.Validate(t.Context(), sign())
	require.NoError(t, err)
	assert.Equal(t, 1, discoveryHits)

	_, err = o.Validate(t.Context(), sign())
	require.NoError(t, err)
	assert.Equal(t, 1, discoveryHits, "provider should be cached")
}

func TestValidate_CancelledContext(t *testing.T) {
	t.Parallel()

	server := newMockServer(t)
	o, _ := NewValidator("gate", []string{server.URL})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := o.Validate(ctx, server.sign(t, server.validClaims("gate")))
	require.Error(t, err)
}
