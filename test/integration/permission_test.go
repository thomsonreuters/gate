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

// TestPermission_RequestNotInPolicy verifies that requests for permissions
// not defined in the policy are rejected with an appropriate error.
func TestPermission_RequestNotInPolicy(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_read.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"issues": "write",
		},
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPermissionNotInPolicy), got.Error.Code)
}

// TestPermission_RequestExceedsOrgMax verifies that requests for permissions
// exceeding the organization's maximum allowed level are rejected.
func TestPermission_RequestExceedsOrgMax(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithMaxPermissions(map[string]string{
		"contents": "read",
		"metadata": "read",
	}))

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPermissionExceedsMax), got.Error.Code)
}

// TestPermission_LevelRead verifies that read-level permissions are
// correctly granted when specified in the policy.
func TestPermission_LevelRead(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_read_metadata_read.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "read", got.Response.Permissions["contents"])
}

// TestPermission_LevelWrite verifies that write-level permissions are
// correctly granted when specified in the policy.
func TestPermission_LevelWrite(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write_metadata_read.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "write", got.Response.Permissions["contents"])
}

// TestPermission_RequestDowngrade verifies that requests for lower
// permission levels than the policy allows are honored.
func TestPermission_RequestDowngrade(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write_metadata_read.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"contents": "read",
		},
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "read", got.Response.Permissions["contents"])
}

// TestPermission_MultiplePermissions verifies that multiple different
// permissions can be requested and granted simultaneously.
func TestPermission_MultiplePermissions(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multiple_permissions.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"contents":      "read",
			"issues":        "write",
			"pull_requests": "read",
		},
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "read", got.Response.Permissions["contents"])
	assert.Equal(t, "write", got.Response.Permissions["issues"])
	assert.Equal(t, "read", got.Response.Permissions["pull_requests"])
}

// TestPermission_RequestExceedsPolicyRejected verifies that requests for
// higher permission levels than the policy allows are rejected.
func TestPermission_RequestExceedsPolicyRejected(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_read_metadata_read.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"contents": "write",
		},
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPermissionExceedsPolicy), got.Error.Code)
}

// TestPermission_EmptyPermissionsUsesPolicy verifies that when no specific
// permissions are requested, all permissions defined in the policy are
// granted.
func TestPermission_EmptyPermissionsUsesPolicy(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write_metadata_read.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "write", got.Response.Permissions["contents"])
	assert.Equal(t, "read", got.Response.Permissions["metadata"])
}

// TestPermission_MultiplePermissionsSomeExceedMax verifies that when multiple
// permissions are requested and some exceed the org maximum, the request fails
// on the first violation.
func TestPermission_MultiplePermissionsSomeExceedMax(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithMaxPermissions(map[string]string{
		"contents": "read",
		"metadata": "read",
		"packages": "read",
	}))

	ctx.SetupPolicy(harness.DefaultRepo, "contents_write_packages_write_metadata_read.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"contents": "read",
			"packages": "write",
		},
	})
	require.NoError(t, err)

	require.NotNil(t, got.Error)
	assert.Equal(t, string(authorizer.ErrPermissionExceedsMax), got.Error.Code)
}

// TestPermission_MaxLevel verifies that write-level (maximum) permissions can be
// requested and granted when allowed by the policy.
func TestPermission_MaxLevel(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx, harness.WithMaxPermissions(map[string]string{
		"contents":            "write",
		"metadata":            "read",
		"repository_projects": "write",
	}))

	ctx.SetupPolicy(harness.DefaultRepo, "max_permission.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "write", got.Response.Permissions["repository_projects"])
}

// TestPermission_PartialPermissionSubset verifies that when only a subset
// of the policy's permissions are requested, only those permissions are
// granted.
func TestPermission_PartialPermissionSubset(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multiple_permissions.tpl.yaml")

	got, err := ctx.Client.Exchange(t.Context(), &harness.ExchangeRequest{
		OIDCToken:        ctx.DefaultToken(),
		TargetRepository: harness.DefaultRepo,
		RequestedPermissions: map[string]string{
			"contents": "read",
		},
	})
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "read", got.Response.Permissions["contents"])

	_, issuesExists := got.Response.Permissions["issues"]
	require.False(t, issuesExists, "expected issues permission to not be included")
	_, prExists := got.Response.Permissions["pull_requests"]
	require.False(t, prExists, "expected pull_requests permission to not be included")
}

// TestPermission_AllPolicyPermissionsGranted verifies that when no specific
// permissions are requested, all permissions from the matching policy are
// granted.
func TestPermission_AllPolicyPermissionsGranted(t *testing.T) {
	ctx := harness.New(t)

	harness.StartServer(t, ctx)

	ctx.SetupPolicy(harness.DefaultRepo, "multiple_permissions.tpl.yaml")

	got, err := ctx.ExchangeDefault()
	require.NoError(t, err)

	require.Nil(t, got.Error)
	assert.Equal(t, "write", got.Response.Permissions["contents"])
	assert.Equal(t, "write", got.Response.Permissions["issues"])
	assert.Equal(t, "write", got.Response.Permissions["pull_requests"])
}
