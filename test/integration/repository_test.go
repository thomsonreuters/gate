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
	"github.com/thomsonreuters/gate/internal/sts"
	"github.com/thomsonreuters/gate/internal/sts/authorizer"
	"github.com/thomsonreuters/gate/test/integration/harness"
)

// TestRepository_InvalidFormat verifies that invalid repository formats are
// rejected. The service validates repository format at the request level for
// clearly malformed inputs like empty strings and strings without the
// owner/repo separator slash.
func TestRepository_InvalidFormat(t *testing.T) {
	tests := []struct {
		name       string
		repository string
	}{
		{name: "no slash", repository: "invalid"},
		{name: "empty repository", repository: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupDefaultPolicy()

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: tt.repository,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, sts.ErrInvalidRequest, got.Error.Code)
		})
	}
}

// TestRepository_MalformedPathsFail verifies that repositories with
// malformed paths are rejected at the request validation level.
func TestRepository_MalformedPathsFail(t *testing.T) {
	tests := []struct {
		name       string
		repository string
	}{
		{name: "too many slashes", repository: "a/b/c"},
		{name: "only slash", repository: "/"},
		{name: "missing owner", repository: "/repo"},
		{name: "missing repo name", repository: "owner/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupDefaultPolicy()

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: tt.repository,
			})
			require.NoError(t, err)

			require.NotNil(t, got.Error)
			assert.Equal(t, sts.ErrInvalidRequest, got.Error.Code)
		})
	}
}

// TestRepository_CrossOrgAccessRequiresPolicy verifies that accessing a
// repository in a different organization requires a valid trust policy in the
// target repository. This test demonstrates that cross-org access fails at the
// policy loading phase when the policy isn't configured for the target
// repository.
func TestRepository_CrossOrgAccessRequiresPolicy(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupDefaultPolicy()

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: "otherorg/otherrepo",
	})
	require.NoError(t, err)

	// Cross-org access fails at policy loading when target has no policy
	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPolicyLoadFailed), got.Error.Code)
}

// TestRepository_SpecialCharactersInName verifies that repositories with valid
// special characters in their names (hyphens, underscores, periods) are handled
// correctly by the service.
func TestRepository_SpecialCharactersInName(t *testing.T) {
	tests := []struct {
		name       string
		repository string
	}{
		{
			name:       "hyphenated name",
			repository: "example-org/my-repo",
		},
		{
			name:       "underscored name",
			repository: "example-org/my_repo",
		},
		{
			name:       "dotted name",
			repository: "example-org/my.repo",
		},
		{
			name:       "complex name",
			repository: "example-org/my-repo_v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := harness.New(t)

			harness.StartServer(t, ctx)

			ctx.SetupPolicy(tt.repository, "contents_read_metadata_read.tpl.yaml")

			got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
				OIDCToken:        ctx.DefaultToken(),
				TargetRepository: tt.repository,
			})
			require.NoError(t, err)

			require.Nil(t, got.Error)
			assert.NotEmpty(t, got.Response.Token)
		})
	}
}
