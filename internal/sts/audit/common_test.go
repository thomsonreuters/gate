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

package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validGrantedEntry() *AuditEntry {
	return &AuditEntry{
		RequestID:        "req-123",
		Timestamp:        time.Now().Unix(),
		Caller:           "repo:example-org/example-repo:ref:refs/heads/main",
		Claims:           map[string]string{"sub": "repo:example-org/example-repo:ref:refs/heads/main"},
		TargetRepository: "example-org/target-repo",
		PolicyName:       "deploy-prod",
		Permissions:      map[string]string{"contents": "read"},
		Outcome:          OutcomeGranted,
		TokenHash:        "sha256:abc123",
		TTL:              900,
		GitHubClientID:   "client-1",
	}
}

func validDeniedEntry() *AuditEntry {
	return &AuditEntry{
		RequestID:        "req-456",
		Timestamp:        time.Now().Unix(),
		Caller:           "repo:example-org/example-repo:ref:refs/heads/main",
		TargetRepository: "example-org/target-repo",
		PolicyName:       "deploy-prod",
		Outcome:          OutcomeDenied,
		DenyReason:       "no matching policy rule",
	}
}

func TestAuditEntry_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*AuditEntry)
		fails  error
	}{
		{name: "valid_granted"},
		{
			name:   "valid_denied",
			modify: func(e *AuditEntry) { *e = *validDeniedEntry() },
		},
		{
			name:   "missing_request_id",
			modify: func(e *AuditEntry) { e.RequestID = "" },
			fails:  ErrInvalidRequestID,
		},
		{
			name:   "missing_timestamp",
			modify: func(e *AuditEntry) { e.Timestamp = 0 },
			fails:  ErrInvalidTimestamp,
		},
		{
			name:   "missing_caller",
			modify: func(e *AuditEntry) { e.Caller = "" },
			fails:  ErrInvalidCaller,
		},
		{
			name:   "missing_target_repository",
			modify: func(e *AuditEntry) { e.TargetRepository = "" },
			fails:  ErrInvalidTargetRepository,
		},
		{
			name:   "invalid_outcome",
			modify: func(e *AuditEntry) { e.Outcome = "unknown" },
			fails:  ErrInvalidOutcome,
		},
		{
			name:   "empty_outcome",
			modify: func(e *AuditEntry) { e.Outcome = "" },
			fails:  ErrInvalidOutcome,
		},
		{
			name: "denied_missing_deny_reason",
			modify: func(e *AuditEntry) {
				*e = *validDeniedEntry()
				e.DenyReason = ""
			},
			fails: ErrInvalidDenyReason,
		},
		{
			name: "denied_missing_policy_name",
			modify: func(e *AuditEntry) {
				*e = *validDeniedEntry()
				e.PolicyName = ""
			},
		},
		{
			name:   "granted_missing_token_hash",
			modify: func(e *AuditEntry) { e.TokenHash = "" },
			fails:  ErrInvalidTokenHash,
		},
		{
			name:   "granted_zero_ttl",
			modify: func(e *AuditEntry) { e.TTL = 0 },
			fails:  ErrInvalidTTL,
		},
		{
			name:   "granted_negative_ttl",
			modify: func(e *AuditEntry) { e.TTL = -1 },
			fails:  ErrInvalidTTL,
		},
		{
			name:   "granted_missing_policy_name",
			modify: func(e *AuditEntry) { e.PolicyName = "" },
			fails:  ErrInvalidPolicyName,
		},
		{
			name:   "granted_missing_github_client_id",
			modify: func(e *AuditEntry) { e.GitHubClientID = "" },
			fails:  ErrInvalidGitHubClientID,
		},
		{
			name: "granted_with_optional_fields_nil",
			modify: func(e *AuditEntry) {
				e.Claims = nil
				e.Permissions = nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entry := validGrantedEntry()
			if tt.modify != nil {
				tt.modify(entry)
			}

			err := entry.Validate()
			if tt.fails != nil {
				require.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOutcome_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, OutcomeGranted, Outcome("granted"))
	assert.Equal(t, OutcomeDenied, Outcome("denied"))
}
