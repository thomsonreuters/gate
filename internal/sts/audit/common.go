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

// Package audit defines the audit logging interface and entry types.
package audit

import (
	"context"
	"errors"
)

// Outcome represents the result of a token exchange for audit purposes.
type Outcome string

const (
	// OutcomeGranted indicates the token exchange was successful.
	OutcomeGranted Outcome = "granted"
	// OutcomeDenied indicates the token exchange was denied.
	OutcomeDenied Outcome = "denied"
)

var (
	// ErrInvalidRequestID is returned when the audit entry has an empty request_id.
	ErrInvalidRequestID = errors.New("request_id is required")
	// ErrInvalidTimestamp is returned when the audit entry has a zero timestamp.
	ErrInvalidTimestamp = errors.New("timestamp is required")
	// ErrInvalidCaller is returned when the audit entry has an empty caller.
	ErrInvalidCaller = errors.New("caller is required")
	// ErrInvalidTargetRepository is returned when the audit entry has an empty target_repository.
	ErrInvalidTargetRepository = errors.New("target_repository is required")
	// ErrInvalidOutcome is returned when outcome is not granted or denied.
	ErrInvalidOutcome = errors.New("outcome must be granted or denied")
	// ErrInvalidDenyReason is returned when outcome is denied but deny_reason is empty.
	ErrInvalidDenyReason = errors.New("deny_reason is required when outcome is denied")
	// ErrInvalidTokenHash is returned when outcome is granted but token_hash is empty.
	ErrInvalidTokenHash = errors.New("token_hash is required when outcome is granted")
	// ErrInvalidTTL is returned when outcome is granted but ttl is not positive.
	ErrInvalidTTL = errors.New("ttl must be positive when outcome is granted")
	// ErrInvalidPolicyName is returned when outcome is granted but policy_name is empty.
	ErrInvalidPolicyName = errors.New("policy_name is required when outcome is granted")
	// ErrInvalidGitHubClientID is returned when outcome is granted but github_client_id is empty.
	ErrInvalidGitHubClientID = errors.New("github_client_id is required when outcome is granted")
)

// AuditEntry records a single token exchange attempt.
type AuditEntry struct {
	RequestID        string            `json:"request_id"                 dynamodbav:"request_id"                 gorm:"column:request_id;not null;uniqueIndex"`
	Timestamp        int64             `json:"timestamp"                  dynamodbav:"timestamp"                  gorm:"column:timestamp;not null"`
	Caller           string            `json:"caller"                     dynamodbav:"caller"                     gorm:"column:caller;not null"`
	Claims           map[string]string `json:"claims,omitempty"           dynamodbav:"claims,omitempty"           gorm:"column:claims;type:jsonb;serializer:json"`
	TargetRepository string            `json:"target_repository"          dynamodbav:"target_repository"          gorm:"column:target_repository;not null"`
	PolicyName       string            `json:"policy_name"                dynamodbav:"policy_name"                gorm:"column:policy_name;not null"`
	Permissions      map[string]string `json:"permissions,omitempty"      dynamodbav:"permissions,omitempty"      gorm:"column:permissions;type:jsonb;serializer:json"`
	Outcome          Outcome           `json:"outcome"                    dynamodbav:"outcome"                    gorm:"column:outcome;not null"`
	DenyReason       string            `json:"deny_reason,omitempty"      dynamodbav:"deny_reason,omitempty"      gorm:"column:deny_reason"`
	TokenHash        string            `json:"token_hash,omitempty"       dynamodbav:"token_hash,omitempty"       gorm:"column:token_hash"`
	TTL              int               `json:"ttl,omitempty"              dynamodbav:"ttl,omitempty"              gorm:"column:ttl"`
	GitHubClientID   string            `json:"github_client_id,omitempty" dynamodbav:"github_client_id,omitempty" gorm:"column:github_client_id"`
	ExpiresAt        int64             `json:"expires_at,omitempty"       dynamodbav:"expires_at,omitempty"       gorm:"-"`
}

// Validate validates the audit entry ensuring all required fields are present.
func (e *AuditEntry) Validate() error {
	if e.RequestID == "" {
		return ErrInvalidRequestID
	}
	if e.Timestamp == 0 {
		return ErrInvalidTimestamp
	}
	if e.Caller == "" {
		return ErrInvalidCaller
	}
	if e.TargetRepository == "" {
		return ErrInvalidTargetRepository
	}
	if e.Outcome != OutcomeGranted && e.Outcome != OutcomeDenied {
		return ErrInvalidOutcome
	}
	if e.Outcome == OutcomeDenied && e.DenyReason == "" {
		return ErrInvalidDenyReason
	}
	if e.Outcome == OutcomeGranted && e.TokenHash == "" {
		return ErrInvalidTokenHash
	}
	if e.Outcome == OutcomeGranted && e.TTL <= 0 {
		return ErrInvalidTTL
	}
	if e.Outcome == OutcomeGranted && e.PolicyName == "" {
		return ErrInvalidPolicyName
	}
	if e.Outcome == OutcomeGranted && e.GitHubClientID == "" {
		return ErrInvalidGitHubClientID
	}
	return nil
}

// AuditEntryBackend is the interface for persisting audit entries.
// Implementations must be safe for concurrent use.
type AuditEntryBackend interface {
	// Log persists the audit entry. Returns an error if validation fails or persistence fails.
	Log(ctx context.Context, entry *AuditEntry) error
	// Close releases resources held by the backend. Safe to call multiple times.
	Close() error
}
