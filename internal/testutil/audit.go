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

package testutil

import (
	"time"

	"github.com/thomsonreuters/gate/internal/sts/audit"
)

// ValidGrantedEntry returns an AuditEntry populated with valid granted-outcome fields.
func ValidGrantedEntry() *audit.AuditEntry {
	return &audit.AuditEntry{
		RequestID:        "req-123",
		Timestamp:        time.Now().Unix(),
		Caller:           "repo:example-org/example-repo:ref:refs/heads/main",
		Claims:           map[string]string{"sub": "repo:example-org/example-repo:ref:refs/heads/main"},
		TargetRepository: "example-org/target-repo",
		PolicyName:       "deploy-prod",
		Permissions:      map[string]string{"contents": "read"},
		Outcome:          audit.OutcomeGranted,
		TokenHash:        "sha256:abc123",
		TTL:              900,
		GitHubClientID:   "client-1",
	}
}

// ValidDeniedEntry returns an AuditEntry populated with valid denied-outcome fields.
func ValidDeniedEntry() *audit.AuditEntry {
	return &audit.AuditEntry{
		RequestID:        "req-456",
		Timestamp:        time.Now().Unix(),
		Caller:           "repo:example-org/example-repo:ref:refs/heads/main",
		TargetRepository: "example-org/target-repo",
		PolicyName:       "deploy-prod",
		Outcome:          audit.OutcomeDenied,
		DenyReason:       "no matching policy rule",
	}
}
