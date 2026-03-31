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
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/thomsonreuters/gate/internal/db"
	"github.com/thomsonreuters/gate/internal/sts/selector"
)

const (
	// DefaultTableName is the default DynamoDB table name when not configured.
	DefaultTableName = "rate_limits"
	// DefaultTTLMinutes is the default TTL in minutes for DynamoDB rate limit items.
	DefaultTTLMinutes = 120
)

// dynamoDBItem is a DynamoDB item for a rate limit state.
type dynamoDBItem struct {
	ClientID    string `dynamodbav:"client_id"`
	Remaining   int    `dynamodbav:"remaining"`
	ResetAt     int64  `dynamodbav:"reset_at"`
	LastUpdated int64  `dynamodbav:"last_updated"`
	ExpiresAt   int64  `dynamodbav:"expires_at,omitempty"`
}

// DynamoDBStore is a DynamoDB-backed implementation of selector.Store.
type DynamoDBStore struct {
	client     db.DynamoDBAPI
	table      string
	ttlMinutes int
}

// NewDynamoDBStore returns a new DynamoDB store.
func NewDynamoDBStore(client db.DynamoDBAPI, table string, ttlMinutes int) *DynamoDBStore {
	if table == "" {
		table = DefaultTableName
	}
	if ttlMinutes <= 0 {
		ttlMinutes = DefaultTTLMinutes
	}
	return &DynamoDBStore{
		client:     client,
		table:      table,
		ttlMinutes: ttlMinutes,
	}
}

func (d *DynamoDBStore) GetState(ctx context.Context, clientID string) (*selector.RateLimitState, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.table,
		Key: map[string]types.AttributeValue{
			"client_id": &types.AttributeValueMemberS{Value: clientID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting item: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, selector.ErrStateNotFound
	}

	var item dynamoDBItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshaling item: %w", err)
	}
	return item.toState(), nil
}

func (d *DynamoDBStore) SetState(ctx context.Context, clientID string, state *selector.RateLimitState) error {
	if state == nil {
		return nil
	}

	item := fromRateLimitState(clientID, state)
	if d.ttlMinutes > 0 {
		item.ExpiresAt = time.Now().Add(time.Duration(d.ttlMinutes) * time.Minute).Unix()
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshaling item: %w", err)
	}

	resetAt := strconv.FormatInt(state.ResetAt.Unix(), 10)
	remaining := strconv.Itoa(state.Remaining)

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.table,
		Item:      av,
		ConditionExpression: aws.String(
			"attribute_not_exists(reset_at) OR " +
				"reset_at < :new_reset_at OR " +
				"(reset_at = :new_reset_at AND remaining > :new_remaining)",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":new_reset_at":  &types.AttributeValueMemberN{Value: resetAt},
			":new_remaining": &types.AttributeValueMemberN{Value: remaining},
		},
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return nil
		}
		return fmt.Errorf("putting item: %w", err)
	}
	return nil
}

func (d *DynamoDBStore) GetAllStates(ctx context.Context) (map[string]*selector.RateLimitState, error) {
	result := make(map[string]*selector.RateLimitState)

	var startKey map[string]types.AttributeValue
	for {
		out, err := d.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         &d.table,
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning: %w", err)
		}

		for _, raw := range out.Items {
			var item dynamoDBItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				slog.WarnContext(ctx, "skipping malformed DynamoDB item", "error", err)
				continue
			}
			result[item.ClientID] = item.toState()
		}

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	return result, nil
}

func (d *DynamoDBStore) Close() error {
	return nil
}

// toState converts the DynamoDB item to a RateLimitState.
func (item *dynamoDBItem) toState() *selector.RateLimitState {
	return &selector.RateLimitState{
		Remaining:   item.Remaining,
		ResetAt:     time.Unix(item.ResetAt, 0),
		LastUpdated: time.Unix(item.LastUpdated, 0),
	}
}

// fromRateLimitState builds a dynamoDBItem from clientID and RateLimitState for PutItem.
func fromRateLimitState(clientID string, s *selector.RateLimitState) *dynamoDBItem {
	return &dynamoDBItem{
		ClientID:    clientID,
		Remaining:   s.Remaining,
		ResetAt:     s.ResetAt.Unix(),
		LastUpdated: s.LastUpdated.Unix(),
	}
}
