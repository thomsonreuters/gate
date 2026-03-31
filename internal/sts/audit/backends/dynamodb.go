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
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/audit"
)

const (
	// DefaultTTLDays is the default TTL in days for DynamoDB audit items when not configured.
	DefaultTTLDays = 90
	// MaxTTLDays is the maximum allowed TTL in days for DynamoDB audit items.
	MaxTTLDays = 365
)

// DynamoDBLogger persists audit entries to DynamoDB.
type DynamoDBLogger struct {
	client  db.DynamoDBAPI
	table   string
	ttlDays int
}

// NewDynamoDBLogger returns a new DynamoDB audit logger.
func NewDynamoDBLogger(client db.DynamoDBAPI, table string, ttlDays int) *DynamoDBLogger {
	return &DynamoDBLogger{
		client:  client,
		table:   table,
		ttlDays: ttlDays,
	}
}

// Log writes the audit entry to DynamoDB.
func (l *DynamoDBLogger) Log(ctx context.Context, entry *audit.AuditEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	clone := *entry
	if l.ttlDays > 0 {
		clone.ExpiresAt = time.Now().UTC().Add(time.Duration(l.ttlDays) * 24 * time.Hour).Unix()
	}

	av, err := attributevalue.MarshalMap(&clone)
	if err != nil {
		return fmt.Errorf("marshaling audit entry: %w", err)
	}

	_, err = l.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &l.table,
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("putting audit entry: %w", err)
	}
	return nil
}

// Close is a no-op for the DynamoDB logger.
func (l *DynamoDBLogger) Close() error {
	return nil
}
