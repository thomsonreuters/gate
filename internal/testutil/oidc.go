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

package testutil

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

// OIDCServer is a minimal OIDC provider for tests.
// It serves discovery, JWKS, and provides token-signing helpers.
type OIDCServer struct {
	URL    string
	server *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer
}

// NewOIDCServer creates a mock OIDC provider backed by the given RSA key.
func NewOIDCServer(t *testing.T, key *rsa.PrivateKey) *OIDCServer {
	t.Helper()

	signingKey := jose.SigningKey{Algorithm: jose.RS256, Key: key}
	signer, err := jose.NewSigner(signingKey, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"))
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	o := &OIDCServer{key: key, signer: signer}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", o.handleDiscovery)
	mux.HandleFunc("/jwks", o.handleJWKS)

	o.server = httptest.NewServer(mux)
	o.URL = o.server.URL
	return o
}

// Close shuts down the mock OIDC provider.
func (o *OIDCServer) Close() {
	if o.server != nil {
		o.server.Close()
	}
}

// SignToken creates a signed JWT with standard claims for the given audience.
func (o *OIDCServer) SignToken(t *testing.T, audience string) string {
	t.Helper()
	now := time.Now()
	return o.SignTokenWithClaims(t, map[string]any{
		"iss":        o.URL,
		"sub":        "repo:example-org/example-repo:ref:refs/heads/main",
		"aud":        audience,
		"exp":        now.Add(1 * time.Hour).Unix(),
		"iat":        now.Unix(),
		"nbf":        now.Unix(),
		"repository": "example-org/example-repo",
		"ref":        "refs/heads/main",
	})
}

// SignTokenWithClaims creates a signed JWT with arbitrary claims.
func (o *OIDCServer) SignTokenWithClaims(t *testing.T, claims map[string]any) string {
	t.Helper()

	builder := josejwt.Signed(o.signer).Claims(claims)
	token, err := builder.Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func (o *OIDCServer) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]string{
		"issuer":                 o.URL,
		"jwks_uri":               o.URL + "/jwks",
		"authorization_endpoint": o.URL + "/authorize",
		"token_endpoint":         o.URL + "/token",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (o *OIDCServer) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwk := jose.JSONWebKey{
		Key:       &o.key.PublicKey,
		KeyID:     "test-key",
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jwks)
}
