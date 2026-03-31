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

//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

func TestOIDC_MalformedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		code  string
	}{
		{name: "empty token", token: "", code: sts.ErrInvalidRequest},
		{name: "not a JWT", token: "not-a-jwt-token", code: sts.ErrInvalidToken}, // #nosec G101 -- test fixture, not a credential
		{name: "incomplete JWT", token: "header.payload", code: sts.ErrInvalidToken},
		{name: "random base64", token: "YWJj.ZGVm.Z2hp", code: sts.ErrInvalidToken}, // #nosec G101 -- test fixture, not a credential
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			ctx.SetupDefaultPolicy()
			harness.StartServer(t, ctx)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        tt.token,
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, tt.code, got.Error.Code)
		})
	}
}

func TestOIDC_ExpiredToken(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	now := time.Now()
	token := ctx.TokenWith(map[string]any{
		"exp": now.Add(-1 * time.Hour).Unix(),
		"iat": now.Add(-2 * time.Hour).Unix(),
		"nbf": now.Add(-2 * time.Hour).Unix(),
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
}

func TestOIDC_NotYetValidToken(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	now := time.Now()
	token := ctx.TokenWith(map[string]any{
		"exp": now.Add(2 * time.Hour).Unix(),
		"nbf": now.Add(1 * time.Hour).Unix(),
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
}

func TestOIDC_UntrustedIssuer(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	token := ctx.TokenWith(map[string]any{
		"iss": "https://untrusted-issuer.example.com",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
}

func TestOIDC_MissingClaims(t *testing.T) {
	tests := []struct {
		name        string
		removeField string
	}{
		{name: "missing issuer", removeField: "iss"},
		{name: "missing expiration", removeField: "exp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			ctx.SetupDefaultPolicy()
			harness.StartServer(t, ctx)

			claims := ctx.DefaultClaims()
			delete(claims, tt.removeField)
			token := ctx.SignTokenWithClaims(claims)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        token,
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
		})
	}
}

func TestOIDC_MissingSubjectSucceeds(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	claims := ctx.DefaultClaims()
	delete(claims, "sub")
	token := ctx.SignTokenWithClaims(claims)

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestOIDC_ValidToken(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
	assert.Equal(t, http.StatusOK, got.StatusCode)
}

func TestOIDC_WrongAudience(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	token := ctx.TokenWith(map[string]any{
		"aud": "https://wrong-audience.example.com",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
}

func TestOIDC_MultipleAudiences(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	token := ctx.TokenWith(map[string]any{
		"aud": []string{"https://other.example.com", ctx.IssuerURL()},
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestOIDC_AdditionalClaims(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupPolicy(harness.DefaultRepo, "environment_claim.tpl.yaml")
	harness.StartServer(t, ctx)

	token := ctx.TokenWith(map[string]any{
		"environment": "production",
		"actor":       "deploy-bot",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestOIDC_FutureIssuedAt(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	now := time.Now()
	token := ctx.TokenWith(map[string]any{
		"exp": now.Add(2 * time.Hour).Unix(),
		"iat": now.Add(1 * time.Hour).Unix(),
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, sts.ErrInvalidToken, got.Error.Code)
}

func TestOIDC_TokenWithAllClaims(t *testing.T) {
	ctx := harness.New(t)
	ctx.SetupDefaultPolicy()
	harness.StartServer(t, ctx)

	token := ctx.TokenWith(map[string]any{
		"jti":                   "unique-token-id-12345",
		"repository_owner":      "example-org",
		"repository_owner_id":   "12345678",
		"repository_id":         "87654321",
		"repository_visibility": "private",
		"ref_type":              "branch",
		"sha":                   "abc123def456",
		"actor":                 "test-user",
		"actor_id":              "11111111",
		"workflow":              "CI",
		"workflow_ref":          "example-org/example-repo/.github/workflows/ci.yml@refs/heads/main",
		"event_name":            "push",
		"run_id":                "9876543210",
		"run_number":            "42",
		"run_attempt":           "1",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
	assert.Equal(t, "read", got.Response.Permissions["contents"])
}
