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
	"errors"
	"fmt"
	"regexp"
)

// Condition checks an OIDC token claim against a regex pattern.
type Condition struct {
	Field    string         `yaml:"field"`
	Pattern  string         `yaml:"pattern"`
	compiled *regexp.Regexp `yaml:"-"`
}

// PolicyRule defines conditions that must be met for a policy to match.
// Logic is "AND" (all conditions must match) or "OR" (at least one). Defaults to "AND".
type PolicyRule struct {
	Name       string      `yaml:"name"`
	Logic      string      `yaml:"logic"`
	Conditions []Condition `yaml:"conditions"`
}

// TrustPolicy defines authorization rules for token exchange requests.
type TrustPolicy struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Issuer      string            `yaml:"issuer"`
	Rules       []PolicyRule      `yaml:"rules"`
	Permissions map[string]string `yaml:"permissions"`
	TokenTTL    int               `yaml:"token_ttl,omitempty"`
}

// PolicyFile represents the trust policy YAML stored in a repository.
type PolicyFile struct {
	Version       string        `yaml:"version"`
	TrustPolicies []TrustPolicy `yaml:"trust_policies"`
}

// Validate validates the policy file structure, version, and ensures all trust policies are valid.
func (f *PolicyFile) Validate() error {
	if f.Version == "" {
		return errors.New("version is required")
	}
	if f.Version != "1.0" {
		return fmt.Errorf("unsupported trust policy version: %s (expected 1.0)", f.Version)
	}
	if len(f.TrustPolicies) == 0 {
		return errors.New("at least one trust policy is required")
	}
	seen := make(map[string]bool, len(f.TrustPolicies))
	for i, p := range f.TrustPolicies {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("policy %d (%s): %w", i, p.Name, err)
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate policy name: %s", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// Validate validates the trust policy structure, ensuring required fields are present and rules are valid.
func (p *TrustPolicy) Validate() error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.Issuer == "" {
		return errors.New("issuer is required")
	}
	if len(p.Rules) == 0 {
		return errors.New("at least one rule is required")
	}
	for i, r := range p.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("rule %d (%s): %w", i, r.Name, err)
		}
	}
	if len(p.Permissions) == 0 {
		return errors.New("at least one permission is required")
	}
	for perm, level := range p.Permissions {
		if !isValidLevel(level) {
			return fmt.Errorf("permission %q has invalid level %q", perm, level)
		}
	}
	return nil
}

// Validate validates the policy rule structure, ensuring it has a name, valid logic, and conditions.
func (r *PolicyRule) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	logic := r.Logic
	if logic == "" {
		logic = "AND"
	}
	if logic != "AND" && logic != "OR" {
		return fmt.Errorf("logic must be AND or OR, got %q", logic)
	}
	if len(r.Conditions) == 0 {
		return errors.New("at least one condition is required")
	}
	for i := range r.Conditions {
		if err := r.Conditions[i].Validate(); err != nil {
			return fmt.Errorf("condition %d: %w", i, err)
		}
	}
	return nil
}

// Validate validates the condition structure and compiles its regex pattern.
func (c *Condition) Validate() error {
	if c.Field == "" {
		return errors.New("field is required")
	}
	if c.Pattern == "" {
		return errors.New("pattern is required")
	}
	compiled, err := regexp.Compile(c.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %q: %w", c.Pattern, err)
	}
	c.compiled = compiled
	return nil
}

// Matches tests if the given value matches the condition's compiled pattern.
func (c *Condition) Matches(value string) bool {
	if c.compiled == nil {
		return false
	}
	return c.compiled.MatchString(value)
}
