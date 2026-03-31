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

package sts

import "fmt"

// Error code constants used in ExchangeError. Returned to API clients in the error_code field.
const (
	ErrInvalidToken        = "INVALID_TOKEN"
	ErrInvalidRequest      = "INVALID_REQUEST"
	ErrRateLimited         = "RATE_LIMITED"
	ErrAppSelectionFailed  = "APP_SELECTION_FAILED"
	ErrClientNotFound      = "GITHUB_CLIENT_NOT_FOUND"
	ErrInternalError       = "INTERNAL_ERROR"
	ErrPolicyLoadFailed    = "POLICY_LOAD_FAILED"
	ErrTrustPolicyNotFound = "TRUST_POLICY_NOT_FOUND"
	ErrRepositoryNotFound  = "REPOSITORY_NOT_FOUND"
	ErrPolicyNotFound      = "POLICY_NOT_FOUND"
	ErrIssuerNotAllowed    = "ISSUER_NOT_ALLOWED"
	ErrGitHubAPIError      = "GITHUB_API_ERROR"
)

// ExchangeError represents a token exchange failure with structured
// error details for API responses.
type ExchangeError struct {
	Code              string `json:"error_code"`                    // Machine-readable code (e.g. INVALID_TOKEN).
	Message           string `json:"error"`                         // Human-readable message.
	Details           string `json:"details,omitempty"`             // Optional additional context.
	RequestID         string `json:"request_id"`                    // Request ID for correlation.
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"` // Set when code is RATE_LIMITED.
}

// Error returns the formatted error message including the error code and details.
func (e *ExchangeError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
