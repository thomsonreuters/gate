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

//go:build integration

package audit

import (
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domain "github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/sts/audit/backends"
	"github.com/thomsonreuters/gate/internal/testutil"
)

const dynamoTable = "audit_logs"

func dynamoItem(t *testing.T, client *dynamodb.Client, id string, timestamp int64) domain.AuditEntry {
	t.Helper()
	table := dynamoTable
	out, err := client.GetItem(t.Context(), &dynamodb.GetItemInput{
		TableName: &table,
		Key: map[string]types.AttributeValue{
			"request_id": &types.AttributeValueMemberS{Value: id},
			"timestamp":  &types.AttributeValueMemberN{Value: strconv.FormatInt(timestamp, 10)},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, out.Item)

	var got domain.AuditEntry
	require.NoError(t, attributevalue.UnmarshalMap(out.Item, &got))
	return got
}

func TestDynamoDB_LogGranted(t *testing.T) {
	t.Parallel()
	client := testutil.NewDynamoDBContainer(t)
	logger := backends.NewDynamoDBLogger(client, dynamoTable, 90)

	entry := grantedEntry("dyn-granted-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	got := dynamoItem(t, client, entry.RequestID, entry.Timestamp)
	assert.Equal(t, entry.RequestID, got.RequestID)
	assert.Equal(t, domain.OutcomeGranted, got.Outcome)
	assert.Equal(t, entry.TokenHash, got.TokenHash)
}

func TestDynamoDB_LogDenied(t *testing.T) {
	t.Parallel()
	client := testutil.NewDynamoDBContainer(t)
	logger := backends.NewDynamoDBLogger(client, dynamoTable, 90)

	entry := deniedEntry("dyn-denied-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	got := dynamoItem(t, client, entry.RequestID, entry.Timestamp)
	assert.Equal(t, domain.OutcomeDenied, got.Outcome)
	assert.Equal(t, "no matching policy rule", got.DenyReason)
}

func TestDynamoDB_TTLAttribute(t *testing.T) {
	t.Parallel()
	client := testutil.NewDynamoDBContainer(t)
	logger := backends.NewDynamoDBLogger(client, dynamoTable, 30)

	entry := grantedEntry("dyn-ttl-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	got := dynamoItem(t, client, entry.RequestID, entry.Timestamp)

	minimum := time.Now().Add(29 * 24 * time.Hour).Unix()
	maximum := time.Now().Add(31 * 24 * time.Hour).Unix()
	assert.Greater(t, got.ExpiresAt, minimum)
	assert.Less(t, got.ExpiresAt, maximum)
}

func TestDynamoDB_NoTTL(t *testing.T) {
	t.Parallel()
	client := testutil.NewDynamoDBContainer(t)
	logger := backends.NewDynamoDBLogger(client, dynamoTable, 0)

	entry := grantedEntry("dyn-nottl-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	got := dynamoItem(t, client, entry.RequestID, entry.Timestamp)
	assert.Zero(t, got.ExpiresAt)
}
