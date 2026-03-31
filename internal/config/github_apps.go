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

import "errors"

var (
	// ErrInvalidGithubAppClientID is returned when a GitHub App entry has an empty client_id.
	ErrInvalidGithubAppClientID = errors.New("client_id is required")
	// ErrInvalidGithubAppPrivateKeyPath is returned when a GitHub App
	// entry has an empty private_key_path.
	ErrInvalidGithubAppPrivateKeyPath = errors.New("private_key_path is required")
	// ErrInvalidGithubAppOrganization is returned when a GitHub App entry has an empty organization.
	ErrInvalidGithubAppOrganization = errors.New("organization is required")
)

// GitHubAppConfig holds a single GitHub App's client ID, key path, and organization.
type GitHubAppConfig struct {
	ClientID       string `mapstructure:"client_id"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
	Organization   string `mapstructure:"organization"`
}

// Validate validates the GitHub App configuration.
func (c *GitHubAppConfig) Validate() error {
	if c.ClientID == "" {
		return ErrInvalidGithubAppClientID
	}
	if c.PrivateKeyPath == "" {
		return ErrInvalidGithubAppPrivateKeyPath
	}
	if c.Organization == "" {
		return ErrInvalidGithubAppOrganization
	}
	return nil
}

// ValidateGitHubApps validates all GitHub App entries and returns the first error encountered.
func ValidateGitHubApps(apps []GitHubAppConfig) error {
	for _, app := range apps {
		if err := app.Validate(); err != nil {
			return err
		}
	}
	return nil
}
