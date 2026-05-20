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

import "time"

const (
	// DefaultBaseURL is the GitHub API base URL when no custom endpoint is set.
	DefaultBaseURL = "https://api.github.com"
	// DefaultTimeout is the HTTP client timeout for GitHub API requests.
	DefaultTimeout = 30 * time.Second
	// DefaultTokenReadyDelay is the pause between minting an installation
	// token and first using it. GitHub's edge infrastructure needs time to
	// replicate new tokens; without a delay, the first request frequently
	// receives a transient 403 "Resource not accessible by integration".
	// Per GitHub Support guidance, 2-3 seconds prevents the majority of
	// replication-lag errors.
	DefaultTokenReadyDelay = 2 * time.Second
)

// Options configures the GitHub App client.
type Options struct {
	BaseURL    string
	ClientID   string // GitHub App Client ID (used as JWT issuer)
	PrivateKey string `json:"-"` // PEM-encoded RSA private key
	Timeout    time.Duration
}
