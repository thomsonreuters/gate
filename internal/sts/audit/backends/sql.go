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

package backends

import (
	"context"
	"fmt"

	"github.com/thomsonreuters/gate/internal/sts/audit"
	"gorm.io/gorm"
)

// tableName is the table name for audit entries.
const tableName = "audit_logs"

// SQLLogger persists audit entries to a SQL database via GORM.
type SQLLogger struct {
	db *gorm.DB
}

// NewSQLLogger returns a new SQL audit logger.
func NewSQLLogger(db *gorm.DB) *SQLLogger {
	return &SQLLogger{db: db}
}

// Log writes the audit entry to the SQL database.
func (l *SQLLogger) Log(ctx context.Context, entry *audit.AuditEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	if err := l.db.WithContext(ctx).Table(tableName).Create(entry).Error; err != nil {
		return fmt.Errorf("creating audit log: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (l *SQLLogger) Close() error {
	sqlDB, err := l.db.DB()
	if err != nil {
		return fmt.Errorf("getting underlying db: %w", err)
	}
	return sqlDB.Close()
}
