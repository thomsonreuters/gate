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

package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DynamoDBAPI defines the subset of DynamoDB operations used by the application.
// The real *dynamodb.Client satisfies this interface. Implementations
// must be safe for concurrent use.
type DynamoDBAPI interface {
	// PutItem writes a single item to the table.
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	// GetItem retrieves a single item by key.
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	// Scan reads items from the table (optionally filtered).
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

var (
	// ErrDynamoDBLoadConfig is returned when the AWS SDK fails to load default config.
	ErrDynamoDBLoadConfig = errors.New("failed to load AWS config")
	// ErrDynamoDBRegionConflict is returned when NewDynamoDB is called
	// with a different region after the singleton was already initialized.
	ErrDynamoDBRegionConflict = errors.New("DynamoDB singleton already initialized with a different region")

	// Singleton state for NewDynamoDB; dynamoMu protects access.
	dynamoDBInstance *dynamodb.Client
	dynamoRegion     string
	dynamoMu         sync.Mutex
)

// NewDynamoDB returns a singleton DynamoDB client.
// If region is empty, the SDK resolves it from the environment/profile.
// Calling with a different non-empty region after initialization returns an error.
func NewDynamoDB(ctx context.Context, region string) (*dynamodb.Client, error) {
	dynamoMu.Lock()
	defer dynamoMu.Unlock()

	if dynamoDBInstance != nil {
		if region != "" && dynamoRegion != "" && region != dynamoRegion {
			return nil, fmt.Errorf("%w: existing=%s, requested=%s", ErrDynamoDBRegionConflict, dynamoRegion, region)
		}
		return dynamoDBInstance, nil
	}

	var opts []func(*awsconfig.LoadOptions) error
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errors.Join(ErrDynamoDBLoadConfig, err)
	}

	dynamoDBInstance = dynamodb.NewFromConfig(cfg)
	dynamoRegion = region

	return dynamoDBInstance, nil
}
