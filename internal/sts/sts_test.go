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

package sts

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/clients/github"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/sts/oidc"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
)

func validClaims() *oidc.Claims {
	return &oidc.Claims{
		Issuer:    "https://token.actions.githubusercontent.com",
		Subject:   "repo:example-org/example-repo:ref:refs/heads/main",
		Audience:  []string{"gate"},
		ExpiresAt: time.Now().Add(10 * time.Minute),
		IssuedAt:  time.Now(),
		Custom: map[string]any{
			"repository": "example-org/example-repo",
			"actor":      "user",
		},
	}
}

func TestNewService_NilConfig(t *testing.T) {
	t.Parallel()
	_, err := NewService(nil, Dependencies{})
	assert.ErrorIs(t, err, ErrNilConfig)
}

func TestNewService_NilSelector(t *testing.T) {
	t.Parallel()
	_, err := NewService(&config.Config{}, Dependencies{})
	assert.ErrorIs(t, err, ErrNilSelector)
}

func TestNewService_NilAudit(t *testing.T) {
	t.Parallel()
	apps := []selector.App{{ClientID: "client-1", Organization: "example-org"}}
	store := backends.NewMemoryStore()
	sel, _ := selector.NewSelector(apps, store)

	_, err := NewService(&config.Config{}, Dependencies{Selector: sel})
	assert.ErrorIs(t, err, ErrNilAudit)
}

func TestNewService_NoApps(t *testing.T) {
	t.Parallel()
	apps := []selector.App{{ClientID: "client-1", Organization: "example-org"}}
	store := backends.NewMemoryStore()
	sel, _ := selector.NewSelector(apps, store)

	_, err := NewService(&config.Config{}, Dependencies{
		Selector: sel,
		Audit:    &audit.MockBackend{},
	})
	assert.ErrorIs(t, err, ErrNoApps)
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	service := &Service{maxTTL: 3600}

	tests := []struct {
		name  string
		req   *ExchangeRequest
		fails string
	}{
		{"empty token", &ExchangeRequest{}, "oidc_token is required"},
		{"empty repo", &ExchangeRequest{OIDCToken: "tok"}, "target_repository is required"},
		{"invalid repo format", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "noslash"}, "owner/repo"},
		{"repo with spaces", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "owner with spaces/repo"}, "owner/repo"},
		{"repo with special chars", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "owner/repo name!"}, "owner/repo"},
		{"negative ttl", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "example-org/example-repo", RequestedTTL: -1}, "negative"},
		{"exceeds max ttl", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "example-org/example-repo", RequestedTTL: 9999}, "exceeds"},
		{"valid", &ExchangeRequest{OIDCToken: "tok", TargetRepository: "example-org/example-repo"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := service.validateRequest(tt.req)
			if tt.fails == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.fails)
			}
		})
	}
}

func TestClaimsToMap(t *testing.T) {
	t.Parallel()
	c := validClaims()
	m := claimsToMap(c)

	assert.Equal(t, c.Issuer, m["iss"])
	assert.Equal(t, c.Subject, m["sub"])
	assert.Equal(t, c.Audience, m["aud"])
	assert.Equal(t, "example-org/example-repo", m["repository"])
}

func TestFlattenClaims(t *testing.T) {
	t.Parallel()
	c := validClaims()
	m := flattenClaims(c)

	assert.Equal(t, c.Issuer, m["iss"])
	assert.Equal(t, c.Subject, m["sub"])
	assert.Equal(t, "example-org/example-repo", m["repository"])
}

func TestCapExpiry(t *testing.T) {
	t.Parallel()
	now := time.Now()

	far := now.Add(2 * time.Hour)
	capped := capExpiry(far, 300)
	assert.True(t, capped.Before(far))

	near := now.Add(1 * time.Minute)
	notCapped := capExpiry(near, 3600)
	assert.Equal(t, near, notCapped)
}

func TestExchangeError_Error(t *testing.T) {
	t.Parallel()

	e := &ExchangeError{Code: "TEST", Message: "msg"}
	assert.Equal(t, "TEST: msg", e.Error())

	e.Details = "detail"
	assert.Equal(t, "TEST: msg (detail)", e.Error())
}

func newTestService(t *testing.T) (*Service, *audit.MockBackend) {
	t.Helper()

	client := &github.MockClient{}
	auditor := &audit.MockBackend{}

	apps := []selector.App{{ClientID: "client-1", Organization: "example-org"}}
	store := backends.NewMemoryStore()
	sel, _ := selector.NewSelector(apps, store)

	service := &Service{
		oidc:         &oidc.Validator{},
		selector:     sel,
		audit:        auditor,
		clients:      map[string]github.ClientIface{"client-1": client},
		tokenTracker: NewTokenTracker(),
		maxTTL:       3600,
		logger:       slog.Default(),
	}
	return service, auditor
}

func TestExchange_EmptyRequest(t *testing.T) {
	t.Parallel()

	service, auditor := newTestService(t)

	_, err := service.Exchange(t.Context(), "req-1", &ExchangeRequest{})
	require.Error(t, err)

	var exchangeErr *ExchangeError
	require.ErrorAs(t, err, &exchangeErr)
	assert.Equal(t, ErrInvalidRequest, exchangeErr.Code)
	assert.Equal(t, "req-1", exchangeErr.RequestID)
	auditor.AssertNotCalled(t, "Log", mock.Anything, mock.Anything)
}

func TestExchange_InvalidRepoFormat(t *testing.T) {
	t.Parallel()

	service, auditor := newTestService(t)

	_, err := service.Exchange(t.Context(), "req-2", &ExchangeRequest{
		OIDCToken:        "tok",
		TargetRepository: "noslash",
	})

	var exchangeErr *ExchangeError
	require.ErrorAs(t, err, &exchangeErr)
	assert.Equal(t, ErrInvalidRequest, exchangeErr.Code)
	assert.Contains(t, exchangeErr.Details, "owner/repo")
	auditor.AssertNotCalled(t, "Log", mock.Anything, mock.Anything)
}

func TestClaimsToMap_NilCustom(t *testing.T) {
	t.Parallel()
	c := &oidc.Claims{
		Issuer:  "https://issuer",
		Subject: "sub",
	}
	m := claimsToMap(c)
	assert.Equal(t, "https://issuer", m["iss"])
	assert.Equal(t, "sub", m["sub"])
}

func TestFlattenClaims_NilCustom(t *testing.T) {
	t.Parallel()
	c := &oidc.Claims{
		Issuer:  "https://issuer",
		Subject: "sub",
	}
	m := flattenClaims(c)
	assert.Equal(t, "https://issuer", m["iss"])
	assert.Equal(t, "sub", m["sub"])
}

func TestFlattenClaims_NestedValues(t *testing.T) {
	t.Parallel()
	c := &oidc.Claims{
		Issuer:  "https://issuer",
		Subject: "sub",
		Custom: map[string]any{
			"repository": "org/repo",
			"nested":     map[string]any{"key": "val"},
			"number":     42,
		},
	}
	m := flattenClaims(c)
	assert.Equal(t, "org/repo", m["repository"])
}

func TestCapExpiry_ZeroTTL(t *testing.T) {
	t.Parallel()
	far := time.Now().Add(2 * time.Hour)
	result := capExpiry(far, 0)
	assert.True(t, result.Before(far), "zero TTL caps to now()")
	assert.WithinDuration(t, time.Now(), result, time.Second)
}

func TestValidateRequest_MaxTTLBoundary(t *testing.T) {
	t.Parallel()
	service := &Service{maxTTL: 3600}

	err := service.validateRequest(&ExchangeRequest{
		OIDCToken:        "tok",
		TargetRepository: "org/repo",
		RequestedTTL:     3600,
	})
	assert.NoError(t, err, "TTL at exactly max should be allowed")
}

func TestRevokeExpiredTokens_Success(t *testing.T) {
	t.Parallel()

	client := &github.MockClient{}
	client.On("RevokeToken", mock.Anything, "token-a").Return(nil)
	client.On("RevokeToken", mock.Anything, "token-b").Return(nil)

	tracker := NewTokenTracker()
	tracker.Record("hash-a", "token-a", time.Now().Add(-time.Minute))
	tracker.Record("hash-b", "token-b", time.Now().Add(-time.Minute))
	tracker.Record("hash-c", "token-c", time.Now().Add(time.Hour))

	service := &Service{
		clients:      map[string]github.ClientIface{"client-1": client},
		tokenTracker: tracker,
		logger:       slog.Default(),
	}

	service.revokeExpiredTokens(t.Context())

	client.AssertNumberOfCalls(t, "RevokeToken", 2)

	expired := tracker.GetExpired()
	assert.Empty(t, expired, "revoked tokens should be removed")
}

func TestRevokeExpiredTokens_PartialFailure(t *testing.T) {
	t.Parallel()

	client := &github.MockClient{}
	client.On("RevokeToken", mock.Anything, "token-ok").Return(nil)
	client.On("RevokeToken", mock.Anything, "token-fail").Return(errors.New("revoke failed"))

	tracker := NewTokenTracker()
	tracker.Record("hash-ok", "token-ok", time.Now().Add(-time.Minute))
	tracker.Record("hash-fail", "token-fail", time.Now().Add(-time.Minute))

	service := &Service{
		clients:      map[string]github.ClientIface{"client-1": client},
		tokenTracker: tracker,
		logger:       slog.Default(),
	}

	service.revokeExpiredTokens(t.Context())

	client.AssertNumberOfCalls(t, "RevokeToken", 2)

	expired := tracker.GetExpired()
	require.Len(t, expired, 1, "failed token should remain in tracker")
	assert.Equal(t, "token-fail", expired["hash-fail"])
}

func TestRevokeExpiredTokens_Empty(t *testing.T) {
	t.Parallel()

	client := &github.MockClient{}

	service := &Service{
		clients:      map[string]github.ClientIface{"client-1": client},
		tokenTracker: NewTokenTracker(),
		logger:       slog.Default(),
	}

	service.revokeExpiredTokens(t.Context())

	client.AssertNotCalled(t, "RevokeToken", mock.Anything, mock.Anything)
}

func TestRevokeExpiredTokens_RetryOnNextRun(t *testing.T) {
	t.Parallel()

	client := &github.MockClient{}
	client.On("RevokeToken", mock.Anything, "token-1").Return(errors.New("network error")).Once()
	client.On("RevokeToken", mock.Anything, "token-1").Return(nil).Once()

	tracker := NewTokenTracker()
	tracker.Record("hash-1", "token-1", time.Now().Add(-time.Minute))

	service := &Service{
		clients:      map[string]github.ClientIface{"client-1": client},
		tokenTracker: tracker,
		logger:       slog.Default(),
	}

	service.revokeExpiredTokens(t.Context())
	expired := tracker.GetExpired()
	require.Len(t, expired, 1, "failed token stays for retry")

	service.revokeExpiredTokens(t.Context())
	expired = tracker.GetExpired()
	assert.Empty(t, expired, "token should be removed after successful retry")

	client.AssertNumberOfCalls(t, "RevokeToken", 2)
}
