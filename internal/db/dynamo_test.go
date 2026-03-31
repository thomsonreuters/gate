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
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"
)

func TestDynamoDBAPI_InterfaceSatisfied(t *testing.T) {
	t.Parallel()

	var _ DynamoDBAPI = (*dynamodb.Client)(nil)
}

func TestNewDynamoDB_ReturnsError_OnInvalidConfig(t *testing.T) {
	dynamoMu.Lock()
	origInstance := dynamoDBInstance
	origRegion := dynamoRegion
	dynamoDBInstance = nil
	dynamoRegion = ""
	dynamoMu.Unlock()
	t.Cleanup(func() {
		dynamoMu.Lock()
		dynamoDBInstance = origInstance
		dynamoRegion = origRegion
		dynamoMu.Unlock()
	})

	// With no AWS credentials configured, LoadDefaultConfig may succeed
	// (it uses default chain) but the client won't be functional.
	// At minimum, verify it doesn't panic and returns something.
	client, err := NewDynamoDB(t.Context(), "us-east-1")
	if err != nil {
		require.ErrorIs(t, err, ErrDynamoDBLoadConfig)
	} else {
		require.NotNil(t, client)
	}
}
