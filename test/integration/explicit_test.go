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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/authorizer"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

// TestExplicit_PolicyNameProvided verifies that when a specific policy name
// is requested, that policy is selected and applied even when multiple
// policies match the conditions.
func TestExplicit_PolicyNameProvided(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "explicit_production_development.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		PolicyName:       "production",
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "production", got.Response.MatchedPolicy)
}

// TestExplicit_PolicyNameNotFound verifies that when a non-existent policy
// name is requested, the service returns a policy not found error.
func TestExplicit_PolicyNameNotFound(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "explicit_production_only.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		PolicyName:       "nonexistent",
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPolicyNotFound), got.Error.Code)
}

// TestExplicit_PolicyNameSelectsCorrectPolicy verifies that specifying a
// policy name correctly selects that policy and applies its permissions, not a
// different matching policy.
func TestExplicit_PolicyNameSelectsCorrectPolicy(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multi_policy_readonly_readwrite.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		PolicyName:       "readwrite",
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "readwrite", got.Response.MatchedPolicy)
}

// TestExplicit_RequireExplicitPolicyWithoutName verifies that when the
// server requires explicit policy names and no policy name is provided, the
// request is rejected.
func TestExplicit_RequireExplicitPolicyWithoutName(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithRequireExplicitPolicy(true))

	ctx.SetupPolicy(harness.DefaultRepo, "contents_read.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPolicyNameRequired), got.Error.Code)
}

// TestExplicit_CaseSensitivePolicyNames verifies that policy names are
// case-sensitive when selecting a specific policy.
func TestExplicit_CaseSensitivePolicyNames(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		matches bool
	}{
		{
			name:    "uppercase Production matches",
			policy:  "Production",
			matches: true,
		},
		{
			name:    "lowercase production matches",
			policy:  "production",
			matches: true,
		},
		{
			name:    "mixed case does not match",
			policy:  "PRODUCTION",
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupPolicy(harness.DefaultRepo, "explicit_case_sensitivity.tpl.yaml")

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: harness.DefaultRepo,
				PolicyName:       tt.policy,
			})
			require.NoError(t, err)

			if tt.matches {
				require.Nil(t, got.Error)
				assert.Equal(t, tt.policy, got.Response.MatchedPolicy)
			} else {
				require.NotNil(t, got.Error)
				assert.Equal(t, string(authorizer.ErrPolicyNotFound), got.Error.Code)
			}
		})
	}
}

// TestExplicit_SpecialCharactersInPolicyNames verifies that policy names with
// special characters like hyphens and underscores are handled correctly.
func TestExplicit_SpecialCharactersInPolicyNames(t *testing.T) {
	tests := []struct {
		name   string
		policy string
	}{
		{
			name:   "hyphenated policy name",
			policy: "deploy-prod",
		},
		{
			name:   "underscored policy name",
			policy: "deploy_staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupPolicy(harness.DefaultRepo, "explicit_special_chars.tpl.yaml")

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: harness.DefaultRepo,
				PolicyName:       tt.policy,
			})
			require.NoError(t, err)

			require.Nil(t, got.Error)
			assert.Equal(t, tt.policy, got.Response.MatchedPolicy)
		})
	}
}

// TestExplicit_EmptyPolicyNameWithExplicitRequired verifies that an empty
// policy name is rejected when explicit policy is required.
func TestExplicit_EmptyPolicyNameWithExplicitRequired(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithRequireExplicitPolicy(true))

	ctx.SetupPolicy(harness.DefaultRepo, "multi_policy_readonly_readwrite.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		PolicyName:       "",
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPolicyNameRequired), got.Error.Code)
}

// TestExplicit_MultiplePoliciesFirstByOrder verifies that when no explicit
// policy is specified and require_explicit_policy is false, the first
// matching policy is selected.
func TestExplicit_MultiplePoliciesFirstByOrder(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithRequireExplicitPolicy(false))

	ctx.SetupPolicy(harness.DefaultRepo, "multi_policy_readonly_readwrite.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "readonly", got.Response.MatchedPolicy)
}
