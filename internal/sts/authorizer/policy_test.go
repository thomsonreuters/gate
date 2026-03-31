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

package authorizer

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/clients/github"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
)

func TestPolicyFile_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		pf, err := parseAndValidate(load(t, "policy_success.yaml"))
		require.NoError(t, err)
		assert.Len(t, pf.TrustPolicies, 1)
	})

	t.Run("missing_version", func(t *testing.T) {
		t.Parallel()
		_, err := parseAndValidate(load(t, "policy_missing_version.yaml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "version is required")
	})

	t.Run("bad_version", func(t *testing.T) {
		t.Parallel()
		_, err := parseAndValidate(load(t, "policy_bad_version.yaml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})

	t.Run("no_policies", func(t *testing.T) {
		t.Parallel()
		_, err := parseAndValidate(load(t, "policy_no_policies.yaml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one trust policy")
	})

	t.Run("duplicate_name", func(t *testing.T) {
		t.Parallel()
		_, err := parseAndValidate(load(t, "policy_duplicate_name.yaml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
	})

	t.Run("invalid_regex", func(t *testing.T) {
		t.Parallel()
		_, err := parseAndValidate(load(t, "policy_invalid_regex.yaml"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pattern")
	})
}

func TestPolicyCache_GetSet(t *testing.T) {
	t.Parallel()
	c := newPolicyCache(time.Minute)

	assert.Nil(t, c.get("key"))

	p := &PolicyFile{Version: "1.0"}
	c.set("key", p)
	assert.Equal(t, p, c.get("key"))
}

func TestPolicyCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	c := newPolicyCache(50 * time.Millisecond)

	p := &PolicyFile{Version: "1.0"}
	c.set("key", p)
	assert.NotNil(t, c.get("key"))

	time.Sleep(100 * time.Millisecond)
	assert.Nil(t, c.get("key"))
}

func TestPolicyCache_LRUEviction(t *testing.T) {
	t.Parallel()
	c := newPolicyCache(10 * time.Minute)

	p := &PolicyFile{Version: "1.0"}
	for i := range maxPolicyCacheEntries {
		c.set(fmt.Sprintf("key%d", i), p)
	}
	assert.Len(t, c.entries, maxPolicyCacheEntries)

	c.set("new-key", p)
	assert.Len(t, c.entries, maxPolicyCacheEntries)
	assert.NotNil(t, c.get("new-key"))
	assert.Nil(t, c.get("key0"))
}

func TestPolicyCache_CachedHit(t *testing.T) {
	t.Parallel()

	data := load(t, "policy_success.yaml")
	m := &github.MockClient{}
	m.On("GetContents", mock.Anything, mock.Anything, ".github/gate/trust-policy.yaml").
		Return(data, nil)
	m.On("GetContents", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("%w: not found", github.ErrFileNotFound))

	app := selector.App{ClientID: "client-1", Organization: "example-org"}
	sel, _ := selector.NewSelector([]selector.App{app}, backends.NewMemoryStore())
	cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})

	authorizer, err := NewAuthorizer(cfg, sel, map[string]github.ClientIface{"client-1": m}, slog.Default())
	require.NoError(t, err)

	req := &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	}

	r1 := authorizer.Authorize(t.Context(), req)
	require.True(t, r1.Allowed)

	r2 := authorizer.Authorize(t.Context(), req)
	require.True(t, r2.Allowed)

	m.AssertNumberOfCalls(t, "GetContents", 1)
}

func TestFetchPolicy_YMLFallback(t *testing.T) {
	t.Parallel()
	app := selector.App{ClientID: "client-1", Organization: "example-org"}
	sel, _ := selector.NewSelector([]selector.App{app}, backends.NewMemoryStore())

	data := load(t, "policy_success.yaml")
	m := &github.MockClient{}
	m.On("GetContents", mock.Anything, mock.Anything, ".github/gate/trust-policy.yaml").
		Return(nil, fmt.Errorf("%w: not found", github.ErrFileNotFound))
	m.On("GetContents", mock.Anything, mock.Anything, ".github/gate/trust-policy.yml").
		Return(data, nil)

	cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
	cfg.TrustPolicyPath = ".github/gate/trust-policy" // extension-less to trigger .yml fallback
	authorizer, err := NewAuthorizer(cfg, sel, map[string]github.ClientIface{"client-1": m}, slog.Default())
	require.NoError(t, err)

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	require.True(t, result.Allowed, "expected yml fallback to work, got: %v", result.DenyReason)
}

func TestAuthorize_CancelledContext(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})

	app := selector.App{ClientID: "client-1", Organization: "example-org"}
	sel, _ := selector.NewSelector([]selector.App{app}, backends.NewMemoryStore())

	m := &github.MockClient{}
	m.On("GetContents", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, context.Canceled)
	authorizer, err := NewAuthorizer(cfg, sel, map[string]github.ClientIface{"client-1": m}, slog.Default())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := authorizer.Authorize(ctx, &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPolicyLoadFailed, result.DenyReason.Code)
}

func TestExtensionVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "yaml extension used as-is",
			path: ".github/gate/trust-policy.yaml",
			want: []string{".github/gate/trust-policy.yaml"},
		},
		{
			name: "yml extension used as-is",
			path: ".github/gate/trust-policy.yml",
			want: []string{".github/gate/trust-policy.yml"},
		},
		{
			name: "no extension produces both variants",
			path: ".github/gate/trust-policy",
			want: []string{".github/gate/trust-policy.yaml", ".github/gate/trust-policy.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extensionVariants(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
