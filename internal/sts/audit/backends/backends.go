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

// Package backends provides audit logging backend implementations.
package backends

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/audit"
)

// NewBackend creates an audit.AuditEntryBackend from the application config.
func NewBackend(ctx context.Context, cfg *config.Config) (audit.AuditEntryBackend, error) {
	switch cfg.Audit.Backend {
	case config.AuditBackendSQL:
		gormDB, err := db.NewDB(ctx, cfg.Audit.SQL.DSN, config.AuditBackendSQL)
		if err != nil {
			return nil, fmt.Errorf("creating sql db: %w", err)
		}
		return NewSQLLogger(gormDB), nil
	case config.AuditBackendDynamoDB:
		client, err := db.NewDynamoDB(ctx, cfg.AWSRegion)
		if err != nil {
			return nil, fmt.Errorf("creating dynamodb client: %w", err)
		}
		return NewDynamoDBLogger(client, cfg.Audit.DynamoDB.TableName, cfg.Audit.DynamoDB.TTLDays), nil
	default:
		return NewConsoleLogger(slog.Default()), nil
	}
}
