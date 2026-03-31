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
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

func dynamoDBItemAttrs(
	clientID string,
	remaining int,
	resetAt, updated int64,
) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"client_id":    &types.AttributeValueMemberS{Value: clientID},
		"remaining":    &types.AttributeValueMemberN{Value: strconv.Itoa(remaining)},
		"reset_at":     &types.AttributeValueMemberN{Value: strconv.FormatInt(resetAt, 10)},
		"last_updated": &types.AttributeValueMemberN{Value: strconv.FormatInt(updated, 10)},
	}
}

func TestDynamoDBStore_Compiles(t *testing.T) {
	t.Parallel()
	var _ selector.Store = (*DynamoDBStore)(nil)
}

func TestNewDynamoDBStore_Defaults(t *testing.T) {
	t.Parallel()

	store := NewDynamoDBStore(&db.MockDynamoDB{}, "", 0)
	assert.Equal(t, DefaultTableName, store.table)
	assert.Equal(t, DefaultTTLMinutes, store.ttlMinutes)
}

func TestNewDynamoDBStore_Custom(t *testing.T) {
	t.Parallel()

	store := NewDynamoDBStore(&db.MockDynamoDB{}, "my-table", 60)
	assert.Equal(t, "my-table", store.table)
	assert.Equal(t, 60, store.ttlMinutes)
}

func TestDynamoDBStore_State_Found(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(time.Hour).Unix()
	updated := time.Now().Unix()

	m := &db.MockDynamoDB{}
	m.On("GetItem", mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{
			Item: dynamoDBItemAttrs("app1", 50, resetAt, updated),
		}, nil)
	store := NewDynamoDBStore(m, "t", 120)

	state, err := store.GetState(t.Context(), "app1")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 50, state.Remaining)
	assert.Equal(t, resetAt, state.ResetAt.Unix())
	assert.Equal(t, updated, state.LastUpdated.Unix())
}

func TestDynamoDBStore_State_NotFound(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("GetItem", mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: nil}, nil)
	store := NewDynamoDBStore(m, "t", 120)

	_, err := store.GetState(t.Context(), "missing")
	require.ErrorIs(t, err, selector.ErrStateNotFound)
}

func TestDynamoDBStore_State_Error(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("GetItem", mock.Anything, mock.Anything).
		Return(nil, errors.New("timeout"))
	store := NewDynamoDBStore(m, "t", 120)

	_, err := store.GetState(t.Context(), "app1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting item")
}

func TestDynamoDBStore_SetState_Success(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	store := NewDynamoDBStore(m, "my-table", 120)

	state := &selector.RateLimitState{
		Remaining:   99,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}

	before := time.Now()
	require.NoError(t, store.SetState(t.Context(), "app1", state))
	m.AssertCalled(t, "PutItem", mock.Anything, mock.Anything)

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	assert.Equal(t, "my-table", *captured.TableName)

	var item dynamoDBItem
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &item))
	assert.Equal(t, "app1", item.ClientID)
	assert.Equal(t, 99, item.Remaining)
	assert.Equal(t, state.ResetAt.Unix(), item.ResetAt)

	expectedMin := before.Add(120 * time.Minute).Unix()
	assert.GreaterOrEqual(t, item.ExpiresAt, expectedMin)

	require.NotNil(t, captured.ConditionExpression)
	assert.Contains(t, *captured.ConditionExpression, "attribute_not_exists(reset_at)")
	assert.Contains(t, *captured.ConditionExpression, "reset_at < :new_reset_at")
	assert.Contains(t, *captured.ConditionExpression, "remaining > :new_remaining")

	require.Contains(t, captured.ExpressionAttributeValues, ":new_reset_at")
	require.Contains(t, captured.ExpressionAttributeValues, ":new_remaining")
}

func TestDynamoDBStore_SetState_NilNoOp(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	store := NewDynamoDBStore(m, "t", 120)

	require.NoError(t, store.SetState(t.Context(), "app1", nil))
	m.AssertNotCalled(t, "PutItem", mock.Anything, mock.Anything)
}

func TestDynamoDBStore_SetState_Error(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(nil, errors.New("throttled"))
	store := NewDynamoDBStore(m, "t", 120)

	state := &selector.RateLimitState{
		Remaining:   10,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}
	err := store.SetState(t.Context(), "app1", state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "putting item")
}

func TestDynamoDBStore_SetState_StaleWriteDiscarded(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{
			Message: aws.String("conditional check failed"),
		})
	store := NewDynamoDBStore(m, "t", 120)

	state := &selector.RateLimitState{
		Remaining:   100,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}
	err := store.SetState(t.Context(), "app1", state)
	require.NoError(t, err, "ConditionalCheckFailedException should be silently discarded")
}

func TestDynamoDBStore_SetState_NoTTL(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("PutItem", mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)
	store := &DynamoDBStore{client: m, table: "t", ttlMinutes: 0}

	state := &selector.RateLimitState{
		Remaining:   10,
		ResetAt:     time.Now().Add(time.Hour),
		LastUpdated: time.Now(),
	}
	require.NoError(t, store.SetState(t.Context(), "app1", state))

	captured, ok := m.Calls[0].Arguments.Get(1).(*dynamodb.PutItemInput)
	require.True(t, ok)
	var item dynamoDBItem
	require.NoError(t, attributevalue.UnmarshalMap(captured.Item, &item))
	assert.Zero(t, item.ExpiresAt)
}

func TestDynamoDBStore_GetAllStates(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(time.Hour).Unix()
	updated := time.Now().Unix()

	m := &db.MockDynamoDB{}
	m.On("Scan", mock.Anything, mock.Anything).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				dynamoDBItemAttrs("app1", 10, resetAt, updated),
				dynamoDBItemAttrs("app2", 20, resetAt, updated),
			},
		}, nil)
	store := NewDynamoDBStore(m, "t", 120)

	all, err := store.GetAllStates(t.Context())
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, 10, all["app1"].Remaining)
	assert.Equal(t, 20, all["app2"].Remaining)
}

func TestDynamoDBStore_AllStates_Error(t *testing.T) {
	t.Parallel()

	m := &db.MockDynamoDB{}
	m.On("Scan", mock.Anything, mock.Anything).
		Return(nil, errors.New("access denied"))
	store := NewDynamoDBStore(m, "t", 120)

	_, err := store.GetAllStates(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scanning")
}

func TestDynamoDBStore_Close(t *testing.T) {
	t.Parallel()
	require.NoError(t, NewDynamoDBStore(&db.MockDynamoDB{}, "t", 120).Close())
}
