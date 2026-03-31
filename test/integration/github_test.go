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
	"github.com/thomsonreuters/gate/internal/sts/authorizer"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

func TestGitHub_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name       string
		serverOpts []harness.ServerOption
		setup      func(*harness.Context)
		targetRepo string
		code   authorizer.ErrorCode
	}{
		{
			name: "installation not found",
			setup: func(ctx *harness.Context) {
				ctx.SetupDefaultPolicy()
				ctx.GitHub.SetError("/repos/example-org/example-repo/installation", 404, "Not Found")
			},
			targetRepo: harness.DefaultRepo,
			code:   authorizer.ErrRepositoryNotFound,
		},
		{
			name: "token creation error",
			setup: func(ctx *harness.Context) {
				ctx.SetupDefaultPolicy()
				ctx.GitHub.SetError("/app/installations/123456/access_tokens", 500, "Internal Server Error")
			},
			targetRepo: harness.DefaultRepo,
			code:   authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "rate limited",
			setup: func(ctx *harness.Context) {
				ctx.SetupDefaultPolicy()
				ctx.GitHub.SetRateLimit("/app/installations/123456/access_tokens", 60)
			},
			targetRepo: harness.DefaultRepo,
			code:   authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "multiple apps wrong org",
			serverOpts: []harness.ServerOption{
				harness.WithGitHubApps([]*harness.TestApp{
					{ClientID: "client-1", Organization: "example-org"},
					{ClientID: "client-2", Organization: "other-org"},
				}),
			},
			setup: func(ctx *harness.Context) {
				ctx.SetupPolicy("third-org/some-repo", "contents_read_metadata_read.tpl.yaml")
			},
			targetRepo: "third-org/some-repo",
			code:   authorizer.ErrPolicyLoadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx, tt.serverOpts...)

			tt.setup(ctx)

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: tt.targetRepo,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, string(tt.code), got.Error.Code)
		})
	}
}

func TestGitHub_MultipleAppsCorrectOrg(t *testing.T) {
	ctx := harness.New(t)

	app1 := &harness.TestApp{ClientID: "client-1", Organization: "example-org"}
	app2 := &harness.TestApp{ClientID: "client-2", Organization: "other-org"}

	harness.StartServer(t, ctx, harness.WithGitHubApps([]*harness.TestApp{app1, app2}))

	ctx.GitHub.SetPolicy("other-org/repo", ctx.LoadTemplate("policies", "contents_read_metadata_read.tpl.yaml"))
	ctx.GitHub.SetInstallation("other-org/repo", 789012)

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: "other-org/repo",
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestGitHub_SingleAppBasicSuccess(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
	assert.True(t, ctx.GitHub.WasRequested("/app/installations/123456/access_tokens"))
}

func TestGitHub_TokenCreationSuccess(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestGitHub_TokenWithCustomPermissions(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write_metadata_read.tpl.yaml")
	ctx.GitHub.SetToken(harness.DefaultInstallationID, "ghs_custom_token", map[string]string{
		"contents": "write",
		"metadata": "read",
	}, time.Now().Add(1*time.Hour))

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "write", got.Response.Permissions["contents"])
	assert.Equal(t, "read", got.Response.Permissions["metadata"])
}

func TestGitHub_MultipleRequestsSameInstallation(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	for range 3 {
		got, err := ctx.ExchangeDefault()
		require.NoError(t, err)

		require.Nil(t, got.Error)
		assert.NotEmpty(t, got.Response.Token)
		assert.Equal(t, "default", got.Response.MatchedPolicy)
	}
}
