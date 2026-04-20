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
	"fmt"
	"sort"
	"strings"
)

const (
	repositoryFormat    = "%s/%s"
	repositorySeparator = "/"
)

// parseRepository splits an "owner/repo" string into its components.
func parseRepository(repository string) (owner, repo string, err error) {
	parts := strings.Split(repository, repositorySeparator)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidRepository, repository)
	}
	return parts[0], parts[1], nil
}

// constructRepository joins owner and repo into the canonical "owner/repo" form.
func constructRepository(owner, repo string) string {
	return fmt.Sprintf(repositoryFormat, owner, repo)
}

// permissionsKey returns a stable string for a permissions map, suitable
// for use as part of a singleflight or cache key.
func permissionsKey(perms map[string]string) string {
	keys := make([]string, 0, len(perms))
	for k := range perms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(perms[k])
	}
	return b.String()
}
