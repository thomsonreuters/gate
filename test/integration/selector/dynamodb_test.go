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

package selector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/selector/backends"
	"github.com/thomsonreuters/gate/internal/testutil"
)

const dynamoTable = "rate_limits"
const dynamoTTL = 120

func newDynamoDBStore(t *testing.T) *backends.DynamoDBStore {
	t.Helper()
	return backends.NewDynamoDBStore(testutil.NewDynamoDBContainer(t), dynamoTable, dynamoTTL)
}

func TestDynamoDB_Lifecycle(t *testing.T) {
	t.Parallel()
	storeLifecycle(t, newDynamoDBStore(t))
}

func TestDynamoDB_ConditionalWrite(t *testing.T) {
	t.Parallel()
	storeConditionalWrite(t, newDynamoDBStore(t))
}

func TestDynamoDB_NewWindowOverwrites(t *testing.T) {
	t.Parallel()
	storeNewWindow(t, newDynamoDBStore(t))
}

func TestDynamoDB_GetAllStates(t *testing.T) {
	t.Parallel()
	storeGetAll(t, newDynamoDBStore(t))
}

func TestDynamoDB_TTLAttribute(t *testing.T) {
	t.Parallel()
	store := backends.NewDynamoDBStore(testutil.NewDynamoDBContainer(t), dynamoTable, 60)
	ctx := t.Context()

	state := rateLimitState(100, 1)
	require.NoError(t, store.SetState(ctx, "ttl-app", state))

	got, err := store.GetState(ctx, "ttl-app")
	require.NoError(t, err)
	assert.Equal(t, 100, got.Remaining)
}
