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

func TestDiscovery_PolicyLoadFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*harness.Context)
		code  authorizer.ErrorCode
	}{
		{
			name: "no policy file",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
			},
			code: authorizer.ErrTrustPolicyNotFound,
		},
		{
			name: "API error",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
				ctx.GitHub.SetError("/repos/example-org/example-repo/contents", 500, "Internal Server Error")
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "rate limit",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
				ctx.GitHub.SetError("/repos/example-org/example-repo/contents", 429, "Rate limit exceeded")
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "forbidden",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
				ctx.GitHub.SetError("/repos/example-org/example-repo/contents", 403, "Resource not accessible")
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "empty policy file",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetPolicy(harness.DefaultRepo, "")
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "unauthorized",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
				ctx.GitHub.SetError("/repos/example-org/example-repo/contents", 401, "Unauthorized")
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
		{
			name: "service unavailable",
			setup: func(ctx *harness.Context) {
				ctx.GitHub.SetInstallation(harness.DefaultRepo, harness.DefaultInstallationID)
				ctx.GitHub.SetError("/repos/example-org/example-repo/contents", 503, "Service Unavailable")
			},
			code: authorizer.ErrPolicyLoadFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)
			harness.StartServer(t, ctx)

			tt.setup(ctx)

			got, err := ctx.ExchangeDefault()
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, string(tt.code), got.Error.Code)
		})
	}
}

func TestDiscovery_PolicyWithTimeout(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_read_metadata_read.tpl.yaml")
	ctx.GitHub.SetLatency(100 * time.Millisecond)

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestDiscovery_CrossOrgAccessWithPolicy(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "cross_org_access.tpl.yaml")

	token := ctx.TokenWith(map[string]any{
		"sub":        "repo:otherorg/workflow-repo:ref:refs/heads/main",
		"repository": "otherorg/workflow-repo",
	})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.NotEmpty(t, got.Response.Token)
}

func TestDiscovery_MultiplePoliciesInFile(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "deeply_nested.tpl.yaml")

	token := ctx.TokenWith(map[string]any{"environment": "development"})

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        token,
		TargetRepository: harness.DefaultRepo,
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "policy-one", got.Response.MatchedPolicy)
}

func TestDiscovery_ConcurrentRequests(t *testing.T) {
	ctx := harness.New(t)
	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	token := ctx.DefaultToken()

	const numRequests = 5
	results := make(chan *harness.Result, numRequests)
	errors := make(chan error, numRequests)

	for range numRequests {
		go func() {
			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        token,
				TargetRepository: harness.DefaultRepo,
			})
			if err != nil {
				errors <- err
				return
			}
			results <- got
		}()
	}

	for i := range numRequests {
		select {
		case err := <-errors:
			require.NoError(t, err)
		case got := <-results:
			require.Nil(t, got.Error)
			assert.NotEmpty(t, got.Response.Token)
		case <-time.After(30 * time.Second):
			require.Fail(t, "timeout waiting for response", "request %d timed out", i)
		}
	}
}
