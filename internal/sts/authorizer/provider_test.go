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

package authorizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaimString(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"repository": "example-org/example-repo",
		"app_metadata": map[string]any{
			"plan":  "premium",
			"roles": []any{"admin", "editor"},
			"preferences": map[string]any{
				"theme":         "dark",
				"notifications": true,
			},
		},
	}

	tests := []struct {
		name  string
		field string
		want  string
		ok    bool
	}{
		{name: "top-level claim", field: "repository", want: "example-org/example-repo", ok: true},
		{name: "nested claim", field: "app_metadata.plan", want: "premium", ok: true},
		{name: "deeply nested claim", field: "app_metadata.preferences.theme", want: "dark", ok: true},
		{name: "missing top-level claim", field: "missing", ok: false},
		{name: "missing nested claim", field: "app_metadata.missing", ok: false},
		{name: "path descends past an object leaf", field: "repository.nope", ok: false},
		{name: "path descends past a scalar leaf", field: "app_metadata.plan.nope", ok: false},
		{name: "non-string leaf (bool)", field: "app_metadata.preferences.notifications", ok: false},
		{name: "non-string leaf (array)", field: "app_metadata.roles", ok: false},
		{name: "object node is not a string", field: "app_metadata", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := claimString(claims, tt.field)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClaimString_LiteralKeyWinsOverPath(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"app_metadata.theme": "flat",
		"app_metadata": map[string]any{
			"theme": "nested",
		},
	}

	got, ok := claimString(claims, "app_metadata.theme")
	assert.True(t, ok)
	assert.Equal(t, "flat", got)
}
