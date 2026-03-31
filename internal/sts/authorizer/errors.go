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
)

// ErrorCode represents specific authorization failure reasons.
type ErrorCode string

// Authorization denial codes returned in Result.DenyReason.
const (
	ErrIssuerNotAllowed             ErrorCode = "ISSUER_NOT_ALLOWED"
	ErrRequiredClaimMismatch        ErrorCode = "REQUIRED_CLAIM_MISMATCH"
	ErrForbiddenClaimMatched        ErrorCode = "FORBIDDEN_CLAIM_MATCHED"
	ErrTimeRestriction              ErrorCode = "TIME_RESTRICTION"
	ErrPolicyLoadFailed             ErrorCode = "POLICY_LOAD_FAILED"
	ErrTrustPolicyNotFound          ErrorCode = "TRUST_POLICY_NOT_FOUND"
	ErrRepositoryNotFound           ErrorCode = "REPOSITORY_NOT_FOUND"
	ErrPolicyNotFound               ErrorCode = "POLICY_NOT_FOUND"
	ErrNoRulesMatched               ErrorCode = "NO_RULES_MATCHED"
	ErrPermissionNotInPolicy        ErrorCode = "PERMISSION_NOT_IN_POLICY"
	ErrPermissionExceedsPolicy      ErrorCode = "PERMISSION_EXCEEDS_POLICY"
	ErrPermissionExceedsMax         ErrorCode = "PERMISSION_EXCEEDS_ORG_MAX"
	ErrPermissionDenied             ErrorCode = "PERMISSION_DENIED"
	ErrPermissionNotInMaxPermission ErrorCode = "PERMISSION_NOT_IN_MAX_PERMISSIONS"
	ErrNonRepoPermission            ErrorCode = "NON_REPOSITORY_PERMISSION"
	ErrPolicyNameRequired           ErrorCode = "POLICY_NAME_REQUIRED"
)

var (
	// ErrPolicyFileNotFound is returned when the trust policy file
	// does not exist at the expected path.
	ErrPolicyFileNotFound = errors.New("trust policy file not found")
	// ErrRepositoryNotAccessible is returned when the repository is not
	// found or not accessible to the app.
	ErrRepositoryNotAccessible = errors.New("repository not found or not accessible")
)

// DenialError represents an authorization failure with a structured code.
type DenialError struct {
	Code    ErrorCode
	Message string
	Details string
}

// Error returns the formatted error message including the error code and details.
func (e *DenialError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// newDenialError builds a DenialError with the given code, message, and optional details.
func newDenialError(code ErrorCode, message, details string) *DenialError {
	return &DenialError{Code: code, Message: message, Details: details}
}
