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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/clients/github"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/sts/selector"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

const testIssuer = "https://token.actions.githubusercontent.com"

func baseConfig(maxPermissions map[string]string) *config.PolicyConfig {
	return &config.PolicyConfig{
		Version:         "1.0",
		TrustPolicyPath: ".github/gate/trust-policy.yaml",
		DefaultTokenTTL: 900,
		MaxTokenTTL:     3600,
		Providers: []config.ProviderConfig{
			{Issuer: testIssuer, Name: "GitHub Actions"},
		},
		MaxPermissions: maxPermissions,
	}
}

func newMockClient(t *testing.T, policyFile string) *github.MockClient {
	t.Helper()
	m := &github.MockClient{}
	if policyFile != "" {
		data := load(t, policyFile)
		m.On("GetContents", mock.Anything, mock.Anything, ".github/gate/trust-policy.yaml").
			Return(data, nil)
		m.On("GetContents", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("%w: not found", github.ErrFileNotFound))
	} else {
		m.On("GetContents", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("%w: not found", github.ErrFileNotFound))
	}
	return m
}

func buildAuthorizer(t *testing.T, cfg *config.PolicyConfig, policyFile string) *Authorizer {
	t.Helper()
	app := selector.App{ClientID: "client-1", Organization: "example-org"}
	sel, err := selector.NewSelector([]selector.App{app}, backends.NewMemoryStore())
	require.NoError(t, err)

	m := newMockClient(t, policyFile)
	clients := map[string]github.ClientIface{"client-1": m}

	a, err := NewAuthorizer(cfg, sel, clients, slog.Default())
	require.NoError(t, err)
	return a
}

func mainBranchClaims() map[string]any {
	return map[string]any{
		"sub": "repo:example-org/example-repo:ref:refs/heads/main",
		"ref": "refs/heads/main",
	}
}

func TestDenialError_Format(t *testing.T) {
	t.Parallel()

	t.Run("with_details", func(t *testing.T) {
		t.Parallel()
		err := newDenialError(ErrIssuerNotAllowed, "bad issuer", "https://evil.com")
		assert.Equal(t, "ISSUER_NOT_ALLOWED: bad issuer (https://evil.com)", err.Error())
	})

	t.Run("without_details", func(t *testing.T) {
		t.Parallel()
		err := newDenialError(ErrIssuerNotAllowed, "bad issuer", "")
		assert.Equal(t, "ISSUER_NOT_ALLOWED: bad issuer", err.Error())
	})
}

func TestNewAuthorizer_InvalidClaimPattern(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	cfg.Providers[0].RequiredClaims = map[string]string{
		"repo": "[invalid",
	}
	_, err := NewAuthorizer(cfg, nil, nil, slog.Default())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid claim pattern")
}

func TestAuthorize_EmptyIssuer(t *testing.T) {
	t.Parallel()
	authorizer := buildAuthorizer(t, baseConfig(nil), "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrIssuerNotAllowed, result.DenyReason.Code)
}

func TestAuthorize_EmptyRepository(t *testing.T) {
	t.Parallel()
	authorizer := buildAuthorizer(t, baseConfig(nil), "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims: mainBranchClaims(),
		Issuer: testIssuer,
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPolicyLoadFailed, result.DenyReason.Code)
}

func TestLayer1_IssuerNotAllowed(t *testing.T) {
	t.Parallel()
	authorizer := buildAuthorizer(t, baseConfig(nil), "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           "https://unknown-issuer.example.com",
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrIssuerNotAllowed, result.DenyReason.Code)
}

func TestLayer1_RequiredClaimMissing(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	cfg.Providers[0].RequiredClaims = map[string]string{
		"repository_owner": "^example-org$",
	}
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           map[string]any{"sub": "test"},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrRequiredClaimMismatch, result.DenyReason.Code)
}

func TestLayer1_RequiredClaimMismatch(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	cfg.Providers[0].RequiredClaims = map[string]string{
		"repository_owner": "^example-org$",
	}
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           map[string]any{"sub": "test", "repository_owner": "wrongorg"},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrRequiredClaimMismatch, result.DenyReason.Code)
}

func TestLayer1_ForbiddenClaimMatched(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	cfg.Providers[0].ForbiddenClaims = map[string]string{
		"repository": "^example-org/forbidden-repo$",
	}
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           map[string]any{"sub": "test", "repository": "example-org/forbidden-repo"},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrForbiddenClaimMatched, result.DenyReason.Code)
}

func TestLayer1_TimeRestriction_Day(t *testing.T) {
	t.Parallel()
	tomorrow := time.Now().UTC().Weekday() + 1
	if tomorrow > 6 {
		tomorrow = 0
	}

	cfg := baseConfig(nil)
	cfg.Providers[0].TimeRestrictions = &config.TimeRestriction{
		AllowedDays: []config.AllowedDays{config.AllowedDays(tomorrow.String())},
	}
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrTimeRestriction, result.DenyReason.Code)
}

func TestLayer1_TimeRestriction_HoursWraparound(t *testing.T) {
	t.Parallel()
	hour := time.Now().UTC().Hour()
	start := (hour + 6) % 24
	end := (hour - 6 + 24) % 24

	cfg := baseConfig(nil)
	cfg.Providers[0].TimeRestrictions = &config.TimeRestriction{
		AllowedHours: &config.HourRange{Start: start, End: end},
	}
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrTimeRestriction, result.DenyReason.Code)
}

func TestIsHourAllowed_NormalRange(t *testing.T) {
	t.Parallel()
	window := &config.HourRange{Start: 9, End: 17}
	assert.True(t, isHourAllowed(9, window))
	assert.True(t, isHourAllowed(12, window))
	assert.True(t, isHourAllowed(17, window))
	assert.False(t, isHourAllowed(8, window))
	assert.False(t, isHourAllowed(18, window))
}

func TestIsHourAllowed_Wraparound(t *testing.T) {
	t.Parallel()
	window := &config.HourRange{Start: 22, End: 6}
	assert.True(t, isHourAllowed(22, window))
	assert.True(t, isHourAllowed(0, window))
	assert.True(t, isHourAllowed(6, window))
	assert.False(t, isHourAllowed(12, window))
	assert.False(t, isHourAllowed(21, window))
}

func TestLayer2_Success(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
		RequestedTTL:     900,
	})
	require.True(t, result.Allowed, "expected allowed, got: %v", result.DenyReason)
	assert.Equal(t, "test-policy", result.MatchedPolicy)
	assert.Len(t, result.EffectivePermissions, 2)
	assert.Equal(t, 900, result.EffectiveTTL)
}

func TestLayer2_RequestedPermissions(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:               mainBranchClaims(),
		Issuer:               testIssuer,
		TargetRepository:     "example-org/example-repo",
		RequestedPermissions: map[string]string{"contents": "read"},
	})
	require.True(t, result.Allowed, "expected allowed, got: %v", result.DenyReason)
	assert.Equal(t, "read", result.EffectivePermissions["contents"])
	assert.Len(t, result.EffectivePermissions, 1)
}

func TestLayer2_RequestedPermissions_ExceedsPolicy(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:               mainBranchClaims(),
		Issuer:               testIssuer,
		TargetRepository:     "example-org/example-repo",
		RequestedPermissions: map[string]string{"contents": "write"},
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPermissionExceedsPolicy, result.DenyReason.Code)
}

func TestLayer2_NoRulesMatched(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_no_rules_matched.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims: map[string]any{
			"sub": "repo:example-org/example-repo:ref:refs/heads/develop",
			"ref": "refs/heads/develop",
		},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrNoRulesMatched, result.DenyReason.Code)
}

func TestLayer2_TrustPolicyFileNotFound(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	authorizer := buildAuthorizer(t, cfg, "")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrTrustPolicyNotFound, result.DenyReason.Code)
	assert.Equal(t, "trust policy file not found in repository", result.DenyReason.Message)
}

func TestLayer2_RepositoryNotFound(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)

	app := selector.App{ClientID: "client-1", Organization: "example-org"}
	sel, _ := selector.NewSelector([]selector.App{app}, backends.NewMemoryStore())

	m := &github.MockClient{}
	m.On("GetContents", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("getting installation ID: %w: example-org/missing-repo", github.ErrRepositoryNotFound))

	a, err := NewAuthorizer(cfg, sel, map[string]github.ClientIface{"client-1": m}, slog.Default())
	require.NoError(t, err)

	result := a.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/missing-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrRepositoryNotFound, result.DenyReason.Code)
	assert.Equal(t, "repository not found or not accessible", result.DenyReason.Message)
}

func TestLayer2_ExplicitPolicyName(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_explicit_name.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
		PolicyName:       "policy-two",
	})
	require.True(t, result.Allowed, "expected allowed, got: %v", result.DenyReason)
	assert.Equal(t, "policy-two", result.MatchedPolicy)
	assert.Equal(t, "write", result.EffectivePermissions["contents"])
}

func TestLayer2_ExplicitPolicyName_RulesNotMatched(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_production_deploy.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims: map[string]any{
			"sub": "repo:example-org/example-repo:ref:refs/heads/feature",
			"ref": "refs/heads/feature",
		},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
		PolicyName:       "production-deploy",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrNoRulesMatched, result.DenyReason.Code)
	assert.Contains(t, result.DenyReason.Message, "production-deploy")
}

func TestLayer2_ExplicitPolicyName_NotFound(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
		PolicyName:       "nonexistent-policy",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPolicyNotFound, result.DenyReason.Code)
}

func TestLayer2_RequireExplicitPolicy(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(nil)
	cfg.RequireExplicitPolicy = true
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPolicyNameRequired, result.DenyReason.Code)
}

func TestLayer2_PermissionExceedsOrgMax(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_exceeds_org_max.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPermissionExceedsMax, result.DenyReason.Code)
}

func TestLayer2_PermissionDenied_NotInMaxPermissions(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPermissionNotInMaxPermission, result.DenyReason.Code)
}

func TestLayer2_PermissionDenied_ExplicitNone(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "none", "packages": "write"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:           mainBranchClaims(),
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPermissionDenied, result.DenyReason.Code)
}

func TestLayer2_PermissionNotInPolicy(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read", "issues": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_no_rules_matched.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims:               mainBranchClaims(),
		Issuer:               testIssuer,
		TargetRepository:     "example-org/example-repo",
		RequestedPermissions: map[string]string{"issues": "read"},
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, ErrPermissionNotInPolicy, result.DenyReason.Code)
}

func TestLayer2_NonRepositoryPermission(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

	for _, perm := range []string{"members", "organization_hooks", "email_addresses", "profile"} {
		t.Run(perm, func(t *testing.T) {
			t.Parallel()
			result := authorizer.Authorize(t.Context(), &Request{
				Claims:               mainBranchClaims(),
				Issuer:               testIssuer,
				TargetRepository:     "example-org/example-repo",
				RequestedPermissions: map[string]string{perm: "read"},
			})
			assert.False(t, result.Allowed)
			assert.Equal(t, ErrNonRepoPermission, result.DenyReason.Code)
		})
	}
}

func TestLayer2_ORLogic(t *testing.T) {
	t.Parallel()
	cfg := baseConfig(map[string]string{"contents": "read"})
	authorizer := buildAuthorizer(t, cfg, "policy_or_logic.yaml")

	result := authorizer.Authorize(t.Context(), &Request{
		Claims: map[string]any{
			"sub": "repo:example-org/example-repo:ref:refs/heads/develop",
			"ref": "refs/heads/develop",
		},
		Issuer:           testIssuer,
		TargetRepository: "example-org/example-repo",
	})
	require.True(t, result.Allowed, "expected allowed with OR logic, got: %v", result.DenyReason)
}

func TestLayer2_TTLResolution(t *testing.T) {
	t.Parallel()

	t.Run("requested_lower", func(t *testing.T) {
		t.Parallel()
		cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
		authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

		result := authorizer.Authorize(t.Context(), &Request{
			Claims:           mainBranchClaims(),
			Issuer:           testIssuer,
			TargetRepository: "example-org/example-repo",
			RequestedTTL:     600,
		})
		require.True(t, result.Allowed)
		assert.Equal(t, 600, result.EffectiveTTL)
	})

	t.Run("default_ttl", func(t *testing.T) {
		t.Parallel()
		cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
		authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

		result := authorizer.Authorize(t.Context(), &Request{
			Claims:           mainBranchClaims(),
			Issuer:           testIssuer,
			TargetRepository: "example-org/example-repo",
		})
		require.True(t, result.Allowed)
		assert.Equal(t, 900, result.EffectiveTTL)
	})

	t.Run("capped_by_max", func(t *testing.T) {
		t.Parallel()
		cfg := baseConfig(map[string]string{"contents": "write", "packages": "write"})
		cfg.MaxTokenTTL = 1200
		authorizer := buildAuthorizer(t, cfg, "policy_success.yaml")

		result := authorizer.Authorize(t.Context(), &Request{
			Claims:           mainBranchClaims(),
			Issuer:           testIssuer,
			TargetRepository: "example-org/example-repo",
			RequestedTTL:     5000,
		})
		require.True(t, result.Allowed)
		assert.Equal(t, 1200, result.EffectiveTTL)
	})
}
