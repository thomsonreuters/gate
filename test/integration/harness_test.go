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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

func TestHarness_Context(t *testing.T) {
	ctx := harness.New(t)

	require.NotNil(t, ctx.GitHub, "GitHub mock is nil")
	require.NotNil(t, ctx.OIDC, "OIDC mock is nil")
}

func TestHarness_MockGitHub(t *testing.T) {
	ctx := harness.New(t)

	ctx.GitHub.SetPolicy("example-org/example-repo", "test: content")
	ctx.GitHub.SetInstallation("example-org/example-repo", 123456)
	ctx.GitHub.SetToken(123456, "test-token", map[string]string{"contents": "read"}, time.Now().Add(time.Hour))
	ctx.GitHub.SetError("/repos/example-org/blocked", 403, "Forbidden")

	assert.NotEmpty(t, ctx.GitHubAPIURL())
}

func TestHarness_OIDC(t *testing.T) {
	ctx := harness.New(t)

	token := ctx.DefaultToken()
	assert.NotEmpty(t, token, "signed token is empty")

	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	assert.Equal(t, 2, parts, "token should have 3 parts, got %d", parts+1)

	token = ctx.SignTokenWithClaims(map[string]any{
		"iss":        ctx.IssuerURL(),
		"sub":        "repo:example-org/example-repo:ref:refs/heads/main",
		"aud":        ctx.IssuerURL(),
		"exp":        time.Now().Add(time.Hour).Unix(),
		"repository": "example-org/example-repo",
	})
	assert.NotEmpty(t, token, "custom claims token is empty")
}

func TestHarness_FixturePath(t *testing.T) {
	ctx := harness.New(t)

	path := ctx.FixturePath("policies", "valid.yaml")
	assert.NotEmpty(t, path, "fixture path is empty")

	want := "fixtures/policies/valid.yaml"
	assert.GreaterOrEqual(t, len(path), len(want), "fixture path too short: %s", path)
}

func TestHarness_Server(t *testing.T) {
	ctx := harness.New(t)

	ctx.GitHub.SetPolicyFromFile("example-org/example-repo", ctx.FixturePath("policies", "valid.yaml"))
	ctx.GitHub.SetInstallation("example-org/example-repo", 123456)

	harness.StartServer(t, ctx)

	require.NotNil(t, ctx.Client, "client is nil after server start")
}
