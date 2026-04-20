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

package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{name: "valid", input: "example-org/example-repo", wantOwner: "example-org", wantRepo: "example-repo"},
		{name: "no_slash", input: "invalid", wantErr: true},
		{name: "too_many_slashes", input: "org/repo/extra", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "empty_owner", input: "/repo", wantErr: true},
		{name: "empty_repo", input: "owner/", wantErr: true},
		{name: "only_slash", input: "/", wantErr: true},
		{name: "double_slash", input: "//", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			owner, repo, err := parseRepository(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidRepository)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestConstructRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		owner string
		repo  string
		want  string
	}{
		{name: "standard", owner: "example-org", repo: "example-repo", want: "example-org/example-repo"},
		{name: "single_char", owner: "a", repo: "b", want: "a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, constructRepository(tt.owner, tt.repo))
		})
	}
}

func TestParseAndConstructRoundTrip(t *testing.T) {
	t.Parallel()
	input := "my-org/my-repo"
	owner, repo, err := parseRepository(input)
	require.NoError(t, err)
	assert.Equal(t, input, constructRepository(owner, repo))
}

func TestPermissionsKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		perms map[string]string
		want  string
	}{
		{name: "single", perms: map[string]string{"contents": "read"}, want: "contents=read"},
		{name: "multiple_sorted", perms: map[string]string{"contents": "read", "metadata": "read"}, want: "contents=read,metadata=read"},
		{name: "insertion_order_ignored", perms: map[string]string{"pull_requests": "write", "contents": "read"}, want: "contents=read,pull_requests=write"},
		{name: "empty", perms: map[string]string{}, want: ""},
		{name: "nil", perms: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, permissionsKey(tt.perms))
		})
	}
}

func TestPermissionsKey_Stable(t *testing.T) {
	t.Parallel()
	perms := map[string]string{"z": "1", "a": "2", "m": "3"}
	first := permissionsKey(perms)
	for range 100 {
		assert.Equal(t, first, permissionsKey(perms))
	}
}
