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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

// TestSuccess_HappyPath verifies a complete successful token exchange.
// It tests the end-to-end flow with valid OIDC token, matching policy,
// and proper permissions to ensure a GitHub token is returned.
func TestSuccess_HappyPath(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.NotEmpty(t, got.Response.Token)
	assert.Equal(t, "default", got.Response.MatchedPolicy)
	assert.NotEmpty(t, got.Response.RequestID)
}

// TestSuccess_MultiplePoliciesSecondMatches verifies that when multiple
// policies exist, the correct matching policy is selected. It creates three
// policies where only the second one matches the request conditions.
func TestSuccess_MultiplePoliciesSecondMatches(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multi_policy_second_matches.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "second-policy", got.Response.MatchedPolicy)
	assert.Equal(t, "write", got.Response.Permissions["contents"])
}
