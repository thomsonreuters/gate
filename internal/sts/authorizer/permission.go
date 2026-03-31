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

import "fmt"

// levels defines the permission level hierarchy (higher value = more permissive) for comparison.
var levels = map[string]int{
	"none":  0,
	"read":  1,
	"write": 2,
}

// deniedPermissions lists GitHub permissions that are
// org/user/enterprise-scoped; repository tokens cannot grant them.
var deniedPermissions = map[string]string{
	// Organization-scoped
	"custom_properties_for_organizations":         "organization",
	"members":                                     "organization",
	"organization_administration":                 "organization",
	"organization_announcement_banners":           "organization",
	"organization_copilot_seat_management":        "organization",
	"organization_custom_org_roles":               "organization",
	"organization_custom_properties":              "organization",
	"organization_custom_roles":                   "organization",
	"organization_events":                         "organization",
	"organization_hooks":                          "organization",
	"organization_packages":                       "organization",
	"organization_personal_access_token_requests": "organization",
	"organization_personal_access_tokens":         "organization",
	"organization_plan":                           "organization",
	"organization_projects":                       "organization",
	"organization_secrets":                        "organization",
	"organization_self_hosted_runners":            "organization",
	"organization_user_blocking":                  "organization",
	"team_discussions":                            "organization",

	// User-scoped
	"email_addresses":    "user",
	"followers":          "user",
	"git_ssh_keys":       "user",
	"gpg_keys":           "user",
	"interaction_limits": "user",
	"profile":            "user",
	"starring":           "user",

	// Enterprise-scoped
	"enterprise_custom_properties_for_organizations": "enterprise",
}

// isValidLevel returns true if level is none, read, or write.
func isValidLevel(level string) bool {
	_, ok := levels[level]
	return ok
}

// resolvePermissions resolves requested permissions against the matched
// policy and org max_permissions; returns denial on error.
func (a *Authorizer) resolvePermissions(req *Request, policy *TrustPolicy) (map[string]string, *DenialError) {
	requested := req.RequestedPermissions
	if len(requested) == 0 {
		requested = policy.Permissions
	}

	permissions := make(map[string]string, len(requested))
	for perm, level := range requested {
		if scope, ok := deniedPermissions[perm]; ok {
			return nil, newDenialError(ErrNonRepoPermission,
				fmt.Sprintf("permission %q is %s-scoped", perm, scope),
				"repository tokens can only be granted repository-scoped permissions")
		}

		allowed, ok := policy.Permissions[perm]
		if !ok {
			return nil, newDenialError(ErrPermissionNotInPolicy,
				fmt.Sprintf("permission %q not granted by policy", perm),
				fmt.Sprintf("policy: %s", policy.Name))
		}

		if !isLevelAllowed(level, allowed) {
			return nil, newDenialError(ErrPermissionExceedsPolicy,
				fmt.Sprintf("requested %q level %q exceeds policy maximum %q", perm, level, allowed),
				fmt.Sprintf("policy: %s", policy.Name))
		}

		if denial := a.validateOrgMax(perm, level); denial != nil {
			return nil, denial
		}

		permissions[perm] = level
	}

	return permissions, nil
}

// validateOrgMax checks the permission against the org-wide max_permissions map.
// Permissions not listed are denied (allowlist model). "none" explicitly forbids.
func (a *Authorizer) validateOrgMax(perm, level string) *DenialError {
	maxLevel, ok := a.config.MaxPermissions[perm]
	if !ok {
		return newDenialError(ErrPermissionNotInMaxPermission,
			fmt.Sprintf("permission %q is not listed in max_permissions", perm),
			"only permissions explicitly listed in max_permissions are allowed")
	}

	if maxLevel == "none" {
		return newDenialError(ErrPermissionDenied,
			fmt.Sprintf("permission %q is explicitly denied", perm),
			"max_permissions is set to 'none' for this permission")
	}

	if !isLevelAllowed(level, maxLevel) {
		return newDenialError(ErrPermissionExceedsMax,
			fmt.Sprintf("permission %q level %q exceeds org maximum", perm, level),
			fmt.Sprintf("max allowed: %s", maxLevel))
	}

	return nil
}

// isLevelAllowed returns true if the requested level is less than or
// equal to the max level in the hierarchy.
func isLevelAllowed(requested, maxLevel string) bool {
	r, ok1 := levels[requested]
	m, ok2 := levels[maxLevel]
	if !ok1 || !ok2 {
		return requested == maxLevel
	}
	return r <= m
}
