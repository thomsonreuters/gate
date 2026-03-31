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

type matchCase struct {
	name    string
	value   string
	matches bool
}

func claimMatch(t *testing.T, policy, claim string, tests []matchCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx)
			ctx.SetupPolicy(harness.DefaultRepo, policy)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.TokenWith(map[string]any{claim: tt.value}),
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			if tt.matches {
				require.Nil(t, got.Error)
			} else {
				require.NotNil(t, got.Error)
				assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
			}
		})
	}
}

func refMatch(t *testing.T, policy string, tests []matchCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx)
			ctx.SetupPolicy(harness.DefaultRepo, policy)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken: ctx.TokenWith(map[string]any{
					"ref": tt.value,
					"sub": "repo:" + harness.DefaultRepo + ":ref:" + tt.value,
				}),
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			if tt.matches {
				require.Nil(t, got.Error)
			} else {
				require.NotNil(t, got.Error)
				assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
			}
		})
	}
}

// TestEvaluation_NoRulesMatched verifies that when no policy rules match the
// request, an appropriate error is returned.
func TestEvaluation_NoRulesMatched(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "no_match_other_repo.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
}

// TestEvaluation_ANDLogicAllMatch verifies that rules with AND logic succeed
// when all conditions are satisfied.
func TestEvaluation_ANDLogicAllMatch(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "and_logic_main_branch.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "default", got.Response.MatchedPolicy)
}

// TestEvaluation_ANDLogicPartialMatch verifies that rules with AND logic
// fail when not all conditions are satisfied.
func TestEvaluation_ANDLogicPartialMatch(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "and_logic_main_branch_contents_only.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.TokenWith(map[string]any{"ref": "refs/heads/feature", "sub": "repo:example-org/example-repo:ref:refs/heads/feature"}),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
}

// TestEvaluation_ORLogicOneMatch verifies that rules with OR logic succeed
// when at least one condition is satisfied.
func TestEvaluation_ORLogicOneMatch(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "or_logic_multiple_repos.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
}

// TestEvaluation_ORLogicNoneMatch verifies that rules with OR logic fail when
// none of the conditions are satisfied.
func TestEvaluation_ORLogicNoneMatch(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "or_logic_no_match.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
}

// TestEvaluation_RegexWildcard verifies that regex patterns with wildcards
// correctly match against claim values.
func TestEvaluation_RegexWildcard(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "regex_wildcard.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
}

// TestEvaluation_IssuerMismatch verifies that requests with tokens from
// issuers not matching the policy are rejected.
func TestEvaluation_IssuerMismatch(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.GitHub.SetPolicyFromFile(harness.DefaultRepo, ctx.FixturePath("policies", "wrong_issuer.yaml"))
	ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
}

// TestEvaluation_ComplexRegexPatterns verifies that complex regex patterns
// in conditions are evaluated correctly. It tests patterns with groups,
// alternation, and anchors.
func TestEvaluation_ComplexRegexPatterns(t *testing.T) {
	refMatch(t, "complex_regex_patterns.tpl.yaml", []matchCase{
		{name: "main branch matches", value: "refs/heads/main", matches: true},
		{name: "release branch matches", value: "refs/heads/release/v1.0", matches: true},
		{name: "feature branch does not match", value: "refs/heads/feature/new-feature", matches: false},
		{name: "develop branch does not match", value: "refs/heads/develop", matches: false},
	})
}

// TestEvaluation_TagRefMatching verifies that tag references are matched
// correctly against semver-style patterns.
func TestEvaluation_TagRefMatching(t *testing.T) {
	refMatch(t, "tag_refs.tpl.yaml", []matchCase{
		{name: "valid semver tag matches", value: "refs/tags/v1.2.3", matches: true},
		{name: "valid semver tag with higher version matches", value: "refs/tags/v10.20.30", matches: true},
		{name: "invalid tag format does not match", value: "refs/tags/release-1.0", matches: false},
		{name: "branch ref does not match tag pattern", value: "refs/heads/main", matches: false},
	})
}

// TestEvaluation_MultipleRulesFirstMatches verifies that when multiple rules
// exist, the first matching rule is used.
func TestEvaluation_MultipleRulesFirstMatches(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multiple_rules_first_matches.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "default", got.Response.MatchedPolicy)
}

// TestEvaluation_EnvironmentClaimMatching verifies that the environment claim
// from GitHub Actions is correctly matched against policy conditions.
func TestEvaluation_EnvironmentClaimMatching(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		matches     bool
	}{
		{
			name:        "production environment matches",
			environment: "production",
			matches:     true,
		},
		{
			name:        "staging environment does not match",
			environment: "staging",
			matches:     false,
		},
		{
			name:        "empty environment does not match",
			environment: "",
			matches:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupPolicy(harness.DefaultRepo, "environment_claim.tpl.yaml")

			overrides := map[string]any{}
			if tt.environment != "" {
				overrides["environment"] = tt.environment
			}
			token := ctx.TokenWith(overrides)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        token,
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			if tt.matches {
				require.Nil(t, got.Error)
			} else {
				require.NotNil(t, got.Error)
				assert.Equal(t, string(authorizer.ErrNoRulesMatched), got.Error.Code)
			}
		})
	}
}

// TestEvaluation_ActorClaimMatching verifies that the actor claim from GitHub
// Actions is correctly matched against policy conditions.
func TestEvaluation_ActorClaimMatching(t *testing.T) {
	claimMatch(t, "actor_claim.tpl.yaml", "actor", []matchCase{
		{name: "admin user matches", value: "admin-user", matches: true},
		{name: "deploy bot matches", value: "deploy-bot", matches: true},
		{name: "regular user does not match", value: "regular-user", matches: false},
	})
}

// TestEvaluation_WorkflowRefMatching verifies that the job_workflow_ref claim
// from reusable workflows is correctly matched.
func TestEvaluation_WorkflowRefMatching(t *testing.T) {
	claimMatch(t, "workflow_ref_claim.tpl.yaml", "job_workflow_ref", []matchCase{
		{name: "approved workflow matches", value: "example-org/shared-workflows/.github/workflows/deploy.yml@refs/heads/main", matches: true},
		{name: "unapproved workflow does not match", value: "example-org/other-repo/.github/workflows/deploy.yml@refs/heads/main", matches: false},
	})
}

// TestEvaluation_EventNameMatching verifies that the event_name claim is
// correctly matched against policy conditions.
func TestEvaluation_EventNameMatching(t *testing.T) {
	claimMatch(t, "event_name_claim.tpl.yaml", "event_name", []matchCase{
		{name: "push event matches", value: "push", matches: true},
		{name: "workflow_dispatch event matches", value: "workflow_dispatch", matches: true},
		{name: "pull_request event does not match", value: "pull_request", matches: false},
	})
}

// TestEvaluation_SubjectClaimPatternMatching verifies that the sub claim can
// be matched using regex patterns.
func TestEvaluation_SubjectClaimPatternMatching(t *testing.T) {
	claimMatch(t, "subject_claim.tpl.yaml", "sub", []matchCase{
		{name: "main branch subject matches", value: "repo:example-org/example-repo:ref:refs/heads/main", matches: true},
		{name: "release branch subject matches", value: "repo:example-org/example-repo:ref:refs/heads/release/v1.0", matches: true},
		{name: "feature branch subject does not match", value: "repo:example-org/example-repo:ref:refs/heads/feature/new", matches: false},
	})
}

// TestEvaluation_ORLogicMultipleConditions verifies that OR logic with
// multiple conditions correctly matches when any condition is satisfied.
func TestEvaluation_ORLogicMultipleConditions(t *testing.T) {
	refMatch(t, "or_logic_multi_condition.tpl.yaml", []matchCase{
		{name: "main branch matches first OR condition", value: "refs/heads/main", matches: true},
		{name: "develop branch matches second OR condition", value: "refs/heads/develop", matches: true},
		{name: "feature branch does not match any OR condition", value: "refs/heads/feature/test", matches: false},
	})
}

// TestEvaluation_MultiplePoliciesThirdMatches verifies that when multiple
// policies exist, the correct matching policy is selected based on conditions.
func TestEvaluation_MultiplePoliciesThirdMatches(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "deeply_nested.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.TokenWith(map[string]any{"environment": "production"}),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "policy-three", got.Response.MatchedPolicy)
	assert.Equal(t, "write", got.Response.Permissions["packages"])
}
