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
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func TestDynamoDBLogger_Compiles(t *testing.T) {
	t.Parallel()
	var _ audit.AuditEntryBackend = (*DynamoDBLogger)(nil)
}

func TestNewDynamoDBLogger(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	logger := NewDynamoDBLogger(m, "audit-table", 30)

	assert.Equal(t, "audit-table", logger.table)
	assert.Equal(t, 30, logger.ttlDays)
}

func TestDynamoDBLogger_Log_Success(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	logger := NewDynamoDBLogger(m, "audit-table", 0)

	entry := testutil.ValidGrantedEntry()
	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)
	m.AssertCalled(t, "PutItem", mock.Anything, mock.Anything)

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	assert.Equal(t, "audit-table", *captured.TableName)

	var persisted audit.AuditEntry
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &persisted))
	assert.Equal(t, entry.RequestID, persisted.RequestID)
	assert.Equal(t, entry.Caller, persisted.Caller)
	assert.Equal(t, entry.TargetRepository, persisted.TargetRepository)
	assert.Equal(t, entry.PolicyName, persisted.PolicyName)
	assert.Equal(t, entry.Outcome, persisted.Outcome)
	assert.Equal(t, entry.TokenHash, persisted.TokenHash)
	assert.Equal(t, entry.GitHubClientID, persisted.GitHubClientID)
	assert.Equal(t, entry.TTL, persisted.TTL)
	assert.Zero(t, persisted.ExpiresAt, "ExpiresAt should be zero when TTL disabled")
}

func TestDynamoDBLogger_Log_WithTTL(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	logger := NewDynamoDBLogger(m, "audit-table", 90)

	entry := testutil.ValidGrantedEntry()
	before := time.Now().UTC()
	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	var persisted audit.AuditEntry
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &persisted))

	expectedMin := before.Add(90 * 24 * time.Hour).Unix()
	expectedMax := time.Now().UTC().Add(90 * 24 * time.Hour).Unix()
	assert.GreaterOrEqual(t, persisted.ExpiresAt, expectedMin)
	assert.LessOrEqual(t, persisted.ExpiresAt, expectedMax)
}

func TestDynamoDBLogger_Log_PreservesClaims(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	logger := NewDynamoDBLogger(m, "audit-table", 0)

	entry := testutil.ValidGrantedEntry()
	entry.Claims = map[string]string{"sub": "repo:example-org/example-repo", "aud": "gate"}
	entry.Permissions = map[string]string{"contents": "read", "metadata": "read"}

	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	var persisted audit.AuditEntry
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &persisted))
	assert.Equal(t, entry.Claims, persisted.Claims)
	assert.Equal(t, entry.Permissions, persisted.Permissions)
}

func TestDynamoDBLogger_Log_ValidationError(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	logger := NewDynamoDBLogger(m, "audit-table", 0)

	err := logger.Log(t.Context(), &audit.AuditEntry{})
	require.ErrorIs(t, err, audit.ErrInvalidRequestID)
	m.AssertNotCalled(t, "PutItem", mock.Anything, mock.Anything)
}

func TestDynamoDBLogger_Log_PutItemError(t *testing.T) {
	t.Parallel()

	putErr := errors.New("throttled")
	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(nil, putErr)
	logger := NewDynamoDBLogger(m, "audit-table", 0)

	entry := testutil.ValidGrantedEntry()
	err := logger.Log(t.Context(), entry)
	require.Error(t, err)
	assert.ErrorIs(t, err, putErr)
}

func TestDynamoDBLogger_Log_DeniedEntry(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	logger := NewDynamoDBLogger(m, "audit-table", 0)

	entry := testutil.ValidDeniedEntry()
	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	var persisted audit.AuditEntry
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &persisted))
	assert.Equal(t, audit.OutcomeDenied, persisted.Outcome)
	assert.Equal(t, entry.DenyReason, persisted.DenyReason)
	assert.Empty(t, persisted.TokenHash)
	assert.Empty(t, persisted.GitHubClientID)
}

func TestDynamoDBLogger_Close(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	logger := NewDynamoDBLogger(m, "t", 0)
	require.NoError(t, logger.Close())
}

func TestDynamoDBLogger_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 90, DefaultTTLDays)
	assert.Equal(t, 365, MaxTTLDays)
}
