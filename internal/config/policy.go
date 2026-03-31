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

package config

import (
	"errors"
	"fmt"
	"slices"
)

const (
	// KeyPolicyVersion is the Viper key for the policy schema version.
	KeyPolicyVersion = "policy.version"
	// KeyPolicyTrustPolicyPath is the Viper key for the trust policy file path.
	KeyPolicyTrustPolicyPath = "policy.trust_policy_path"
	// KeyPolicyDefaultTokenTTL is the Viper key for the default token TTL in seconds.
	KeyPolicyDefaultTokenTTL = "policy.default_token_ttl"
	// KeyPolicyMaxTokenTTL is the Viper key for the maximum token TTL in seconds.
	KeyPolicyMaxTokenTTL = "policy.max_token_ttl"
	// KeyPolicyRequireExplicitPolicy is the Viper key for requiring an explicit matching policy.
	KeyPolicyRequireExplicitPolicy = "policy.require_explicit_policy"
	// KeyPolicyGitHubAPIBaseURL is the Viper key for the GitHub API base URL.
	KeyPolicyGitHubAPIBaseURL = "policy.github_api_base_url"
	// KeyPolicyGitHubRawBaseURL is the Viper key for the GitHub raw content base URL.
	KeyPolicyGitHubRawBaseURL = "policy.github_raw_base_url"
	// KeyPolicyProviders is the Viper key for the OIDC provider list.
	KeyPolicyProviders = "policy.providers"
	// KeyPolicyMaxPermissions is the Viper key for the maximum allowed permissions map.
	KeyPolicyMaxPermissions = "policy.max_permissions"

	// DefaultPolicyVersion is the default policy schema version.
	DefaultPolicyVersion = "1.0"
	// DefaultPolicyDefaultTokenTTL is the default token TTL in seconds.
	DefaultPolicyDefaultTokenTTL = 900
	// DefaultPolicyMaxTokenTTL is the default maximum token TTL in seconds.
	DefaultPolicyMaxTokenTTL = 3600
	// DefaultGitHubAPIBaseURL is the default GitHub API base URL.
	DefaultGitHubAPIBaseURL = "https://api.github.com"
	// DefaultGitHubRawBaseURL is the default GitHub raw content base URL.
	DefaultGitHubRawBaseURL = "https://raw.githubusercontent.com"
)

var (
	// ErrInvalidStartHour is returned when allowed_hours start is not in 0-23.
	ErrInvalidStartHour = errors.New("start hour must be 0-23")
	// ErrInvalidEndHour is returned when allowed_hours end is not in 0-23.
	ErrInvalidEndHour = errors.New("end hour must be 0-23")
	// ErrInvalidAllowedDays is returned when an allowed day is not a valid weekday name.
	ErrInvalidAllowedDays = errors.New("invalid allowed days")
	// ErrInvalidProviderName is returned when a provider has an empty name.
	ErrInvalidProviderName = errors.New("provider name is required")
	// ErrInvalidProviderIssuer is returned when a provider has an empty issuer.
	ErrInvalidProviderIssuer = errors.New("provider issuer is required")
	// ErrInvalidPolicyVersion is returned when the policy version is empty or not supported.
	ErrInvalidPolicyVersion = errors.New("invalid policy version")
	// ErrInvalidTrustPolicyPath is returned when trust_policy_path is empty.
	ErrInvalidTrustPolicyPath = errors.New("trust policy path is required")
	// ErrInvalidDefaultTokenTTL is returned when default_token_ttl is not positive.
	ErrInvalidDefaultTokenTTL = errors.New("default token TTL must be positive")
	// ErrInvalidMaxTokenTTL is returned when max_token_ttl is not positive.
	ErrInvalidMaxTokenTTL = errors.New("max token TTL must be positive")
	// ErrDefaultTTLExceedsMax is returned when default_token_ttl is greater than max_token_ttl.
	ErrDefaultTTLExceedsMax = errors.New("default token TTL must be less than or equal to max token TTL")
	// ErrInvalidPermissionLevel is returned when a permission level is not none, read, or write.
	ErrInvalidPermissionLevel = errors.New("invalid permission level")
)

// ValidPolicyVersions lists the supported policy schema versions.
var ValidPolicyVersions = []string{
	"1.0",
}

// validPermissionLevels is a map of valid permission levels (none, read, write).
var validPermissionLevels = map[string]bool{
	"none":  true,
	"read":  true,
	"write": true,
}

// AllowedDays is a weekday name for time restrictions (e.g. Monday, Tuesday).
type AllowedDays string

const (
	AllowedDaysMonday    AllowedDays = "Monday"
	AllowedDaysTuesday   AllowedDays = "Tuesday"
	AllowedDaysWednesday AllowedDays = "Wednesday"
	AllowedDaysThursday  AllowedDays = "Thursday"
	AllowedDaysFriday    AllowedDays = "Friday"
	AllowedDaysSaturday  AllowedDays = "Saturday"
	AllowedDaysSunday    AllowedDays = "Sunday"
)

// ValidAllowedDays is the list of allowed weekday values for time restrictions.
var ValidAllowedDays = []AllowedDays{
	AllowedDaysMonday,
	AllowedDaysTuesday,
	AllowedDaysWednesday,
	AllowedDaysThursday,
	AllowedDaysFriday,
	AllowedDaysSaturday,
	AllowedDaysSunday,
}

// HourRange holds start and end hours (0-23) for time-based access restrictions.
type HourRange struct {
	Start int `mapstructure:"start"`
	End   int `mapstructure:"end"`
}

// Validate validates the hour range ensuring Start and End are between 0-23.
func (h *HourRange) Validate() error {
	if h.Start < 0 || h.Start > 23 {
		return ErrInvalidStartHour
	}
	if h.End < 0 || h.End > 23 {
		return ErrInvalidEndHour
	}
	return nil
}

// TimeRestriction holds allowed days and optional hour range for provider access.
type TimeRestriction struct {
	AllowedDays  []AllowedDays `mapstructure:"allowed_days"`
	AllowedHours *HourRange    `mapstructure:"allowed_hours"`
}

// Validate validates the time restriction configuration.
func (t *TimeRestriction) Validate() error {
	if t.AllowedHours != nil {
		if err := t.AllowedHours.Validate(); err != nil {
			return err
		}
	}

	for _, day := range t.AllowedDays {
		if !slices.Contains(ValidAllowedDays, day) {
			return ErrInvalidAllowedDays
		}
	}

	return nil
}

// ProviderConfig holds OIDC provider identity, claims, and optional time restrictions.
type ProviderConfig struct {
	Issuer           string            `mapstructure:"issuer"`
	Name             string            `mapstructure:"name"`
	RequiredClaims   map[string]string `mapstructure:"required_claims"`
	ForbiddenClaims  map[string]string `mapstructure:"forbidden_claims"`
	TimeRestrictions *TimeRestriction  `mapstructure:"time_restrictions"`
}

// Validate validates the OIDC provider configuration.
func (p *ProviderConfig) Validate() error {
	if p.Issuer == "" {
		return ErrInvalidProviderIssuer
	}
	if p.Name == "" {
		return ErrInvalidProviderName
	}
	if p.TimeRestrictions != nil {
		if err := p.TimeRestrictions.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// PolicyConfig holds trust policy path, token TTL limits, providers, and max permissions.
type PolicyConfig struct {
	Version               string            `mapstructure:"version"`
	TrustPolicyPath       string            `mapstructure:"trust_policy_path"`
	DefaultTokenTTL       int               `mapstructure:"default_token_ttl"`
	MaxTokenTTL           int               `mapstructure:"max_token_ttl"`
	RequireExplicitPolicy bool              `mapstructure:"require_explicit_policy"`
	GitHubAPIBaseURL      string            `mapstructure:"github_api_base_url"`
	GitHubRawBaseURL      string            `mapstructure:"github_raw_base_url"`
	Providers             []ProviderConfig  `mapstructure:"providers"`
	MaxPermissions        map[string]string `mapstructure:"max_permissions"`
}

// Issuers returns the issuer URLs from all configured providers.
func (p *PolicyConfig) Issuers() []string {
	issuers := make([]string, 0, len(p.Providers))
	for _, prov := range p.Providers {
		issuers = append(issuers, prov.Issuer)
	}
	return issuers
}

// Validate validates the policy configuration.
func (p *PolicyConfig) Validate() error {
	if p.Version == "" || !slices.Contains(ValidPolicyVersions, p.Version) {
		return ErrInvalidPolicyVersion
	}
	if p.TrustPolicyPath == "" {
		return ErrInvalidTrustPolicyPath
	}
	if p.DefaultTokenTTL <= 0 {
		return ErrInvalidDefaultTokenTTL
	}
	if p.MaxTokenTTL <= 0 {
		return ErrInvalidMaxTokenTTL
	}
	if p.DefaultTokenTTL > p.MaxTokenTTL {
		return ErrDefaultTTLExceedsMax
	}

	for _, provider := range p.Providers {
		if err := provider.Validate(); err != nil {
			return err
		}
	}

	if err := validatePermissions(p.MaxPermissions); err != nil {
		return err
	}

	return nil
}

// validatePermissions checks that all permission levels are none, read, or write.
func validatePermissions(permissions map[string]string) error {
	for permission, level := range permissions {
		if !validPermissionLevels[level] {
			return fmt.Errorf("%w: permission %q has level %q (must be none, read, or write)", ErrInvalidPermissionLevel, permission, level)
		}
	}
	return nil
}
