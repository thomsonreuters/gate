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

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/mock"
)

// MockDynamoDB is a testify mock implementing DynamoDBAPI.
type MockDynamoDB struct {
	mock.Mock
}

//nolint:errcheck // mock: panic on unexpected type is intentional
func (m *MockDynamoDB) PutItem(
	ctx context.Context,
	params *dynamodb.PutItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, params)
	if out := args.Get(0); out != nil {
		return out.(*dynamodb.PutItemOutput), args.Error(1)
	}
	return nil, args.Error(1)
}

//nolint:errcheck // mock: panic on unexpected type is intentional
func (m *MockDynamoDB) GetItem(
	ctx context.Context,
	params *dynamodb.GetItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, params)
	if out := args.Get(0); out != nil {
		return out.(*dynamodb.GetItemOutput), args.Error(1)
	}
	return nil, args.Error(1)
}

//nolint:errcheck // mock: panic on unexpected type is intentional
func (m *MockDynamoDB) Scan(
	ctx context.Context,
	params *dynamodb.ScanInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.ScanOutput, error) {
	args := m.Called(ctx, params)
	if out := args.Get(0); out != nil {
		return out.(*dynamodb.ScanOutput), args.Error(1)
	}
	return nil, args.Error(1)
}
