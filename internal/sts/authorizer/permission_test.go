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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/config"
)

func TestValidateOrgMax(t *testing.T) {
	t.Parallel()
	a := &Authorizer{config: &config.PolicyConfig{
		MaxPermissions: map[string]string{
			"contents": "write",
			"issues":   "read",
			"actions":  "none",
		},
	}}

	require.Nil(t, a.validateOrgMax("contents", "read"), "read within write ceiling")
	require.Nil(t, a.validateOrgMax("contents", "write"), "write within write ceiling")
	require.Nil(t, a.validateOrgMax("issues", "read"), "read within read ceiling")

	d := a.validateOrgMax("issues", "write")
	require.NotNil(t, d)
	assert.Equal(t, ErrPermissionExceedsMax, d.Code, "write exceeds read ceiling")

	d = a.validateOrgMax("actions", "read")
	require.NotNil(t, d)
	assert.Equal(t, ErrPermissionDenied, d.Code, "none means explicitly denied")

	d = a.validateOrgMax("deployments", "read")
	require.NotNil(t, d)
	assert.Equal(t, ErrPermissionNotInMaxPermission, d.Code, "not listed in map")
}

func TestIsLevelAllowed_PermissionHierarchy(t *testing.T) {
	t.Parallel()
	assert.True(t, isLevelAllowed("read", "write"), "read within write ceiling")
	assert.True(t, isLevelAllowed("read", "read"), "read within read ceiling")
	assert.False(t, isLevelAllowed("write", "read"), "write exceeds read ceiling")
	assert.True(t, isLevelAllowed("none", "write"), "none within write ceiling")
	assert.True(t, isLevelAllowed("write", "write"), "write within write ceiling")
}

func TestIsLevelAllowed(t *testing.T) {
	t.Parallel()
	assert.True(t, isLevelAllowed("read", "write"))
	assert.True(t, isLevelAllowed("read", "read"))
	assert.False(t, isLevelAllowed("write", "read"))
	assert.True(t, isLevelAllowed("custom", "custom"))
	assert.False(t, isLevelAllowed("custom", "other"))
}

func TestEvaluateCondition_NonStringClaim(t *testing.T) {
	t.Parallel()
	cond := &Condition{Field: "count", Pattern: ".*", compiled: regexp.MustCompile(".*")}
	assert.False(t, evaluateCondition(cond, map[string]any{"count": 42}))
}

func TestEvaluateCondition_MissingClaim(t *testing.T) {
	t.Parallel()
	cond := &Condition{Field: "ref", Pattern: ".*", compiled: regexp.MustCompile(".*")}
	assert.False(t, evaluateCondition(cond, map[string]any{}))
}

func TestCondition_Matches_NilCompiled(t *testing.T) {
	t.Parallel()
	cond := &Condition{Field: "f", Pattern: "p"}
	assert.False(t, cond.Matches("anything"))
}
