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

const (
	// KeyAuditBackend is the Viper key for the audit backend type (sql or dynamodb).
	KeyAuditBackend = "audit.backend"
	// KeyAuditDynamoDBTableName is the Viper key for the DynamoDB audit table name.
	KeyAuditDynamoDBTableName = "audit.dynamodb.table_name"
	// KeyAuditDynamoDBTTLDays is the Viper key for DynamoDB audit entry TTL in days.
	KeyAuditDynamoDBTTLDays = "audit.dynamodb.ttl_days"
	// KeyAuditSQLDSN is the Viper key for the SQL audit connection DSN.
	KeyAuditSQLDSN = "audit.sql.dsn"
)

const (
	// DefaultDynamoDBTableName is the default DynamoDB table name for audit logs.
	DefaultDynamoDBTableName = "audit_logs"

	// DefaultDynamoDBTTLDays is the default TTL in days for DynamoDB audit entries.
	// 90 days provides sufficient audit history for compliance while managing storage costs.
	DefaultDynamoDBTTLDays = 90

	// MaxDynamoDBTTLDays is the maximum allowed TTL in days for DynamoDB audit entries.
	MaxDynamoDBTTLDays = 365
)

// AuditBackendType identifies the audit storage backend (sql or dynamodb).
type AuditBackendType string

const (
	// AuditBackendSQL uses a SQL database for audit storage.
	AuditBackendSQL AuditBackendType = "sql"
	// AuditBackendDynamoDB uses DynamoDB for audit storage.
	AuditBackendDynamoDB AuditBackendType = "dynamodb"
)

var (
	// ErrInvalidAuditBackendType is returned when the audit backend is not "sql" or "dynamodb".
	ErrInvalidAuditBackendType = errors.New("invalid audit backend type")
	// ErrInvalidSQLDSN is returned when the SQL DSN is empty.
	ErrInvalidSQLDSN = errors.New("SQL DSN is required")
	// ErrInvalidSQLConfig is returned when the audit backend is sql but sql config is missing.
	ErrInvalidSQLConfig = errors.New("sql config is required")
	// ErrInvalidDynamoDBConfig is returned when the audit backend is
	// dynamodb but dynamodb config is missing.
	ErrInvalidDynamoDBConfig = errors.New("dynamodb config is required")
	// ErrInvalidDynamoDBTable is returned when the DynamoDB table name is empty.
	ErrInvalidDynamoDBTable = errors.New("dynamodb table name is required")
	// ErrInvalidDynamoDBTTLDays is returned when TTL days are not in [0, 365].
	ErrInvalidDynamoDBTTLDays = errors.New("TTL days must be between 0 and 365")
)

// AuditDynamoDBConfig holds DynamoDB-specific audit configuration.
type AuditDynamoDBConfig struct {
	TableName string `mapstructure:"table_name"`
	TTLDays   int    `mapstructure:"ttl_days"`
}

// Validate validates the DynamoDB audit configuration.
func (d *AuditDynamoDBConfig) Validate() error {
	if d.TableName == "" {
		return ErrInvalidDynamoDBTable
	}
	if d.TTLDays < 0 || d.TTLDays > MaxDynamoDBTTLDays {
		return ErrInvalidDynamoDBTTLDays
	}
	return nil
}

// AuditSQLConfig holds SQL connection settings for the audit backend.
type AuditSQLConfig struct {
	DSN string `mapstructure:"dsn" json:"-"`
}

// Validate validates the SQL audit configuration.
func (p *AuditSQLConfig) Validate() error {
	if p.DSN == "" {
		return ErrInvalidSQLDSN
	}
	return nil
}

// AuditConfig holds audit backend type and backend-specific settings.
type AuditConfig struct {
	Backend  AuditBackendType     `mapstructure:"backend"`
	DynamoDB *AuditDynamoDBConfig `mapstructure:"dynamodb"`
	SQL      *AuditSQLConfig      `mapstructure:"sql"`
}

// Validate validates the audit configuration based on the selected backend type.
func (a *AuditConfig) Validate() error {
	if a.Backend == "" {
		return nil
	}

	switch a.Backend {
	case AuditBackendSQL:
		if a.SQL == nil {
			return ErrInvalidSQLConfig
		}
		return a.SQL.Validate()
	case AuditBackendDynamoDB:
		if a.DynamoDB == nil {
			return ErrInvalidDynamoDBConfig
		}
		return a.DynamoDB.Validate()
	default:
		return ErrInvalidAuditBackendType
	}
}

// IsMigrationSupported returns true if the audit backend supports database migrations.
func (a *AuditConfig) IsMigrationSupported() bool {
	return a.Backend == AuditBackendSQL
}
