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

// TestParsing_PolicyErrors verifies that policy files with various parsing
// errors are rejected with POLICY_LOAD_FAILED.
func TestParsing_PolicyErrors(t *testing.T) {
	policyErrorCode := string(authorizer.ErrPolicyLoadFailed)

	tests := []struct {
		name    string
		fixture string
	}{
		{name: "invalid YAML", fixture: "invalid_syntax.yaml"},
		{name: "missing version", fixture: "missing_version.yaml"},
		{name: "wrong version", fixture: "wrong_version.yaml"},
		{name: "missing required fields/missing name", fixture: "missing_name.yaml"},
		{name: "missing required fields/missing issuer", fixture: "missing_issuer.yaml"},
		{name: "missing required fields/missing permissions", fixture: "missing_permissions.yaml"},
		{name: "missing required fields/empty rules", fixture: "empty_rules.yaml"},
		{name: "missing required fields/missing conditions", fixture: "missing_conditions.yaml"},
		{name: "invalid permission level", fixture: "invalid_permission_level.yaml"},
		{name: "invalid regex pattern", fixture: "invalid_regex_pattern.yaml"},
		{name: "duplicate policy names", fixture: "duplicate_policy_names.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.GitHub.SetPolicyFromFile(harness.DefaultRepo, ctx.FixturePath("policies", tt.fixture))
			ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: harness.DefaultRepo,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, policyErrorCode, got.Error.Code)
		})
	}
}

// TestParsing_DeeplyNestedStructure verifies that policy files with multiple
// policies and rules are parsed correctly without issues.
func TestParsing_DeeplyNestedStructure(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "deeply_nested.tpl.yaml")

	token := ctx.TokenWith(map[string]any{
		"environment": "staging",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "policy-two", got.Response.MatchedPolicy)
}

// TestParsing_UnicodeContent verifies that policy files containing Unicode
// characters in names and values are parsed correctly.
func TestParsing_UnicodeContent(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "unicode_content.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}
