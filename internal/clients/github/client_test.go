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

package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func TestNewClient(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	tests := []struct {
		name  string
		opts  Options
		fails bool
	}{
		{
			name:  "valid",
			opts:  Options{ClientID: "client-1", PrivateKey: keyPEM},
			fails: false,
		},
		{
			name:  "custom_base_url",
			opts:  Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: "https://custom.github.com/api/v3"},
			fails: false,
		},
		{
			name:  "invalid_key",
			opts:  Options{ClientID: "client-1", PrivateKey: "not-a-key"},
			fails: true,
		},
		{
			name:  "empty_key",
			opts:  Options{ClientID: "client-1", PrivateKey: ""},
			fails: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := New(tt.opts)
			if tt.fails {
				require.Error(t, err)
				assert.Nil(t, c)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, c)
			}
		})
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM})
	require.NoError(t, err)

	client, ok := c.(*Client)
	require.True(t, ok)
	assert.Equal(t, DefaultBaseURL, client.baseURL)
}

func TestGenerateAppJWT(t *testing.T) {
	t.Parallel()
	key, keyPEM := testutil.GenerateRSAKey(t)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM})
	require.NoError(t, err)

	client, ok := c.(*Client)
	require.True(t, ok)
	tokenStr, err := client.generateAppJWT()
	require.NoError(t, err)

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, isRSA := token.Method.(*jwt.SigningMethodRSA); !isRSA {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &key.PublicKey, nil
	})
	require.NoError(t, err)
	assert.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, "client-1", claims["iss"])
	assert.Contains(t, claims, "iat")
	assert.Contains(t, claims, "exp")
}

func TestRequestToken_Success(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/app/installations/12345/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:gosec // G101: test fixture, not real credentials
				"token":      "ghs_test_token_123",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	resp, err := c.RequestToken(t.Context(), &TokenRequest{
		Repository:  "example-org/example-repo",
		Permissions: map[string]string{"contents": "read"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ghs_test_token_123", resp.Token)
	assert.False(t, resp.ExpiresAt.IsZero())
	assert.Equal(t, 2, calls)
}

func TestRequestToken_CachedInstallationID(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/app/installations/12345/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      "ghs_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	ctx := t.Context()
	req := &TokenRequest{Repository: "example-org/example-repo", Permissions: map[string]string{"contents": "read"}}

	_, err = c.RequestToken(ctx, req)
	require.NoError(t, err)
	first := calls

	_, err = c.RequestToken(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, first+1, calls, "second call should skip installation ID lookup")
}

func TestRequestToken_InvalidRepository(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM})
	require.NoError(t, err)

	tests := []struct {
		name string
		repo string
	}{
		{name: "no_slash", repo: "invalid"},
		{name: "too_many_slashes", repo: "org/repo/extra"},
		{name: "empty", repo: ""},
		{name: "empty_owner", repo: "/repo"},
		{name: "empty_repo", repo: "owner/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := c.RequestToken(t.Context(), &TokenRequest{Repository: tt.repo})
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidRepository)
		})
	}
}

func TestRequestToken_ContextCancellation(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = c.RequestToken(ctx, &TokenRequest{
		Repository:  "example-org/example-repo",
		Permissions: map[string]string{"contents": "read"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestRequestToken_InstallationLookupFailure(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/") {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	_, err = c.RequestToken(t.Context(), &TokenRequest{
		Repository:  "example-org/example-repo",
		Permissions: map[string]string{"contents": "read"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting installation ID")
}

func TestRequestToken_TokenCreationFailure(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/app/installations/12345/access_tokens") {
			http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
			return
		}
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	_, err = c.RequestToken(t.Context(), &TokenRequest{
		Repository:  "example-org/example-repo",
		Permissions: map[string]string{"contents": "write"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating installation token")
}

// newTestClient creates a Client with zero tokenReadyDelay for fast tests.
func newTestClient(t *testing.T, opts Options) ClientIface {
	t.Helper()
	c, err := New(opts)
	require.NoError(t, err)
	client, ok := c.(*Client)
	require.True(t, ok)
	client.tokenReadyDelay = 0
	return client
}

func TestGetContents_Success(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:gosec // G101: test fixture, not real credentials
				"token":      "ghs_contents_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/contents/.github/trust-policy.yaml") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":     "file",
				"encoding": "base64",
				"content":  "dmVyc2lvbjogIjEuMCI=", // version: "1.0"
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

	content, err := c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, `version: "1.0"`, string(content))
}

func TestGetContents_FileNotFound(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      "ghs_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

	_, err := c.GetContents(t.Context(), "example-org/example-repo", "nonexistent.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestGetContents_RepositoryNotFound(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

	_, err := c.GetContents(t.Context(), "example-org/nonexistent-repo", ".github/policy.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRepositoryNotFound)
}

func TestGetContents_TokenReadyDelay(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	var contentsAttempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:gosec // G101: test fixture
				"token":      "ghs_contents_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		if strings.Contains(r.URL.Path, "/contents/") {
			contentsAttempts.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":     "file",
				"encoding": "base64",
				"content":  "dmVyc2lvbjogIjEuMCI=",
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)
	client, ok := c.(*Client)
	require.True(t, ok)
	client.tokenReadyDelay = 50 * time.Millisecond

	start := time.Now()
	_, err = c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "should wait for token ready delay")
	assert.Equal(t, int32(1), contentsAttempts.Load(), "should succeed on first attempt after delay")
}

func TestGetContents_TokenReadyDelay_ContextCancellation(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/access_tokens") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      "ghs_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)
	client, ok := c.(*Client)
	require.True(t, ok)
	client.tokenReadyDelay = 5 * time.Second

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err = c.GetContents(ctx, "example-org/example-repo", ".github/trust-policy.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetContents_RetriesWithFreshTokenOnAuthError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		errBody    string
	}{
		{name: "403", statusCode: http.StatusForbidden, errBody: `{"message":"Resource not accessible by integration"}`},
		{name: "401", statusCode: http.StatusUnauthorized, errBody: `{"message":"Bad credentials"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, keyPEM := testutil.GenerateRSAKey(t)

			var tokenCreations atomic.Int32
			revokedToken := atomic.Value{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				if strings.HasSuffix(r.URL.Path, "/installation") {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
					return
				}
				if strings.Contains(r.URL.Path, "/access_tokens") {
					n := tokenCreations.Add(1)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"token":      fmt.Sprintf("ghs_tok_%d", n),
						"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
					})
					return
				}
				if strings.Contains(r.URL.Path, "/contents/") {
					auth := r.Header.Get("Authorization")
					if revoked, ok := revokedToken.Load().(string); ok && strings.Contains(auth, revoked) {
						http.Error(w, tt.errBody, tt.statusCode)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"type":     "file",
						"encoding": "base64",
						"content":  "dmVyc2lvbjogIjEuMCI=",
					})
					return
				}
				http.Error(w, "not found", http.StatusNotFound)
			}))
			t.Cleanup(server.Close)

			c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

			_, err := c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
			require.NoError(t, err)
			assert.Equal(t, int32(1), tokenCreations.Load())

			revokedToken.Store("ghs_tok_1")

			content, err := c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
			require.NoError(t, err, "should succeed after retrying with fresh token")
			assert.Equal(t, `version: "1.0"`, string(content))
			assert.Equal(t, int32(2), tokenCreations.Load(), "should have minted a fresh token")
		})
	}
}

func TestGetContents_Persistent403(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		preSeed         bool
		wantMints       int32
		wantContentHits int32
	}{
		// Pre-seeded: cached token → 403 (retried by retryTransport 4×) → evict → mint fresh → 403 (4×)
		{name: "cached_token_retries_then_fails", preSeed: true, wantMints: 1, wantContentHits: 8},
		// No cache: mint fresh → 403 (retried by retryTransport 4×) → no second attempt
		{name: "fresh_token_does_not_retry", preSeed: false, wantMints: 1, wantContentHits: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, keyPEM := testutil.GenerateRSAKey(t)

			var tokenCreations atomic.Int32
			var contentHits atomic.Int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				if strings.HasSuffix(r.URL.Path, "/installation") {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
					return
				}
				if strings.Contains(r.URL.Path, "/access_tokens") {
					tokenCreations.Add(1)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"token":      fmt.Sprintf("ghs_tok_%d", tokenCreations.Load()),
						"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
					})
					return
				}
				if strings.Contains(r.URL.Path, "/contents/") {
					contentHits.Add(1)
					http.Error(w, `{"message":"Resource not accessible by integration"}`, http.StatusForbidden)
					return
				}
				http.Error(w, "not found", http.StatusNotFound)
			}))
			t.Cleanup(server.Close)

			c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

			if tt.preSeed {
				client, ok := c.(*Client)
				require.True(t, ok)
				client.contentsTokens.set(
					"example-org/example-repo",
					"ghs_stale",
					time.Now().Add(time.Hour),
				)
			}

			_, err := c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "403")
			assert.Equal(t, tt.wantMints, tokenCreations.Load())
			assert.Equal(t, tt.wantContentHits, contentHits.Load())
		})
	}
}

func TestInstallationCache_UpdateExistingAtCapacity(t *testing.T) {
	t.Parallel()
	c := newInstallationCache()

	for i := range maxInstallationCacheEntries {
		c.set(fmt.Sprintf("org%d/repo%d", i, i), int64(i+1000))
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	assert.Len(t, c.entries, maxInstallationCacheEntries)

	c.set("org0/repo0", 9999)
	assert.Len(t, c.entries, maxInstallationCacheEntries, "updating existing key at capacity must not evict")
	assert.Equal(t, int64(9999), c.get("org0/repo0"))
}

func TestGetContents_InvalidRepository(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM})
	require.NoError(t, err)

	_, err = c.GetContents(t.Context(), "invalid", ".github/policy.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRepository)
}

func TestRevokeToken_Success(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	var gotMethod string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	err = c.RevokeToken(t.Context(), "ghs_test_token")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/installation/token", gotPath)
}

func TestRevokeToken_UnauthorizedTreatedAsSuccess(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	err = c.RevokeToken(t.Context(), "ghs_expired_token")
	require.NoError(t, err, "401 means token is already invalid, which achieves revocation")
}

func TestRevokeToken_ServerError(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Internal Server Error"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	err = c.RevokeToken(t.Context(), "ghs_test_token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revoking installation token")
}

func TestRevokeToken_ContextCancellation(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = c.RevokeToken(ctx, "ghs_test_token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestRateLimit_Success(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)
	reset := time.Now().Add(time.Hour).Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rate_limit", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		auth := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(auth, "Bearer ghs_"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"resources":{"core":{"limit":5000,"remaining":4997,"reset":%d}}}`, reset.Unix())
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	got, err := c.RateLimit(t.Context(), "ghs_test_token")
	require.NoError(t, err)
	assert.Equal(t, 4997, got.Remaining)
	assert.Equal(t, reset.Unix(), got.ResetAt.Unix())
}

func TestRateLimit_Error(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	_, err = c.RateLimit(t.Context(), "ghs_test_token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "querying rate limit")
}

func TestRateLimit_NilCore(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"resources":{}}`)
	}))
	t.Cleanup(server.Close)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})
	require.NoError(t, err)

	got, err := c.RateLimit(t.Context(), "ghs_test_token")
	require.NoError(t, err)
	assert.Equal(t, 0, got.Remaining)
	assert.True(t, got.ResetAt.IsZero())
}

func TestToInstallationPermissions(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	c, err := New(Options{ClientID: "client-1", PrivateKey: keyPEM})
	require.NoError(t, err)
	client, ok := c.(*Client)
	require.True(t, ok)

	t.Run("nil_for_empty_map", func(t *testing.T) {
		t.Parallel()
		p, err := client.toInstallationPermissions(map[string]string{})
		require.NoError(t, err)
		assert.Nil(t, p)
	})

	t.Run("nil_for_nil_map", func(t *testing.T) {
		t.Parallel()
		p, err := client.toInstallationPermissions(nil)
		require.NoError(t, err)
		assert.Nil(t, p)
	})

	t.Run("maps_known_permissions", func(t *testing.T) {
		t.Parallel()
		p, err := client.toInstallationPermissions(map[string]string{
			"contents": "read",
			"metadata": "read",
			"issues":   "write",
		})
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "read", *p.Contents)
		assert.Equal(t, "read", *p.Metadata)
		assert.Equal(t, "write", *p.Issues)
		assert.Nil(t, p.Actions)
	})

	t.Run("ignores_unknown_keys", func(t *testing.T) {
		t.Parallel()
		p, err := client.toInstallationPermissions(map[string]string{
			"contents":          "read",
			"nonexistent_field": "write",
		})
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "read", *p.Contents)
	})
}

func TestInstallationCache_GetSet(t *testing.T) {
	t.Parallel()
	c := newInstallationCache()

	assert.Equal(t, int64(0), c.get("example-org/example-repo"))

	c.set("example-org/example-repo", 12345)
	assert.Equal(t, int64(12345), c.get("example-org/example-repo"))
}

func TestInstallationCache_Expiration(t *testing.T) {
	t.Parallel()
	c := newInstallationCache()

	c.set("example-org/example-repo", 12345)
	assert.Equal(t, int64(12345), c.get("example-org/example-repo"))

	c.mu.Lock()
	e := c.entries["example-org/example-repo"]
	e.fetched = time.Now().Add(-25 * time.Hour)
	c.entries["example-org/example-repo"] = e
	c.mu.Unlock()

	assert.Equal(t, int64(0), c.get("example-org/example-repo"))
}

func TestInstallationCache_Eviction(t *testing.T) {
	t.Parallel()
	c := newInstallationCache()

	for i := range maxInstallationCacheEntries {
		c.set(fmt.Sprintf("org%d/repo%d", i, i), int64(i+1000))
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	assert.Len(t, c.entries, maxInstallationCacheEntries)

	c.set("neworg/newrepo", 99999)
	assert.Len(t, c.entries, maxInstallationCacheEntries)
	assert.Equal(t, int64(99999), c.get("neworg/newrepo"))
	assert.Equal(t, int64(0), c.get("org0/repo0"))
}

func TestTokenCache_GetSet(t *testing.T) {
	t.Parallel()
	c := newTokenCache()

	assert.Empty(t, c.get("example-org/example-repo"))

	c.set("example-org/example-repo", "ghs_abc", time.Now().Add(time.Hour))
	assert.Equal(t, "ghs_abc", c.get("example-org/example-repo"))
}

func TestTokenCache_Expiration(t *testing.T) {
	t.Parallel()
	c := newTokenCache()

	c.set("example-org/example-repo", "ghs_abc", time.Now().Add(2*time.Minute))
	assert.Empty(t, c.get("example-org/example-repo"), "token expiring within buffer window should not be served")

	c.set("example-org/example-repo", "ghs_fresh", time.Now().Add(time.Hour))
	assert.Equal(t, "ghs_fresh", c.get("example-org/example-repo"))
}

func TestTokenCache_Delete(t *testing.T) {
	t.Parallel()
	c := newTokenCache()

	c.set("example-org/repo-a", "ghs_abc", time.Now().Add(time.Hour))
	assert.Equal(t, "ghs_abc", c.get("example-org/repo-a"))

	c.delete("example-org/repo-a")
	assert.Empty(t, c.get("example-org/repo-a"))

	c.delete("nonexistent/key")
}

func TestTokenCache_Eviction(t *testing.T) {
	t.Parallel()
	c := newTokenCache()

	for i := range maxTokenCacheEntries {
		c.set(fmt.Sprintf("org%d/repo%d", i, i), fmt.Sprintf("tok_%d", i), time.Now().Add(time.Hour+time.Duration(i)*time.Millisecond))
	}
	assert.Len(t, c.entries, maxTokenCacheEntries)

	c.set("neworg/newrepo", "tok_new", time.Now().Add(time.Hour))
	assert.Len(t, c.entries, maxTokenCacheEntries)
	assert.Equal(t, "tok_new", c.get("neworg/newrepo"))
	assert.Empty(t, c.get("org0/repo0"), "entry with earliest expiry should have been evicted")
}

func TestTokenCache_UpdateExistingAtCapacity(t *testing.T) {
	t.Parallel()
	c := newTokenCache()

	for i := range maxTokenCacheEntries {
		c.set(fmt.Sprintf("org%d/repo%d", i, i), fmt.Sprintf("tok_%d", i), time.Now().Add(time.Hour))
	}
	assert.Len(t, c.entries, maxTokenCacheEntries)

	c.set("org0/repo0", "tok_updated", time.Now().Add(2*time.Hour))
	assert.Len(t, c.entries, maxTokenCacheEntries, "updating existing key at capacity must not evict")
	assert.Equal(t, "tok_updated", c.get("org0/repo0"))
}

func TestGetContents_CachedToken(t *testing.T) {
	t.Parallel()
	_, keyPEM := testutil.GenerateRSAKey(t)

	var tokenCreations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/repos/example-org/example-repo/installation") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12345})
			return
		}

		if strings.Contains(r.URL.Path, "/access_tokens") {
			tokenCreations.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:gosec // G101: test fixture
				"token":      "ghs_contents_token",
				"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}

		if strings.Contains(r.URL.Path, "/contents/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":     "file",
				"encoding": "base64",
				"content":  "dmVyc2lvbjogIjEuMCI=",
			})
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	c := newTestClient(t, Options{ClientID: "client-1", PrivateKey: keyPEM, BaseURL: server.URL})

	_, err := c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, int32(1), tokenCreations.Load())

	_, err = c.GetContents(t.Context(), "example-org/example-repo", ".github/trust-policy.yaml")
	require.NoError(t, err)
	assert.Equal(t, int32(1), tokenCreations.Load(), "second call should reuse cached token")
}

func TestNetworkError(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("connection refused")
	err := &NetworkError{err: originalErr}

	assert.Contains(t, err.Error(), "network error")
	assert.Contains(t, err.Error(), "connection refused")
	require.ErrorIs(t, err, originalErr)

	var target *NetworkError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, originalErr, target.Unwrap())
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		msg    string
	}{
		{name: "404", status: 404, msg: "not found"},
		{name: "500", status: 500, msg: "internal server error"},
		{name: "403", status: 403, msg: "forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := &APIError{statusCode: tt.status, message: tt.msg}

			assert.Equal(t, tt.status, err.StatusCode())
			assert.Contains(t, err.Error(), "GitHub API error")
			assert.Contains(t, err.Error(), fmt.Sprintf("status %d", tt.status))
			assert.Contains(t, err.Error(), tt.msg)

			var target *APIError
			assert.ErrorAs(t, err, &target)
		})
	}
}
