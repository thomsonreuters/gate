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

	"github.com/stretchr/testify/assert"
	"github.com/thomsonreuters/gate/internal/config"
)

func TestNewDB_InvalidBackendType(t *testing.T) {
	mu.Lock()
	origInstance := instance
	instance = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		instance = origInstance
		mu.Unlock()
	})

	_, err := NewDB(t.Context(), "invalid-dsn", config.AuditBackendDynamoDB)
	assert.ErrorIs(t, err, config.ErrInvalidAuditBackendType)
}

func TestNewDB_UnknownBackendType(t *testing.T) {
	mu.Lock()
	origInstance := instance
	instance = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		instance = origInstance
		mu.Unlock()
	})

	_, err := NewDB(t.Context(), "invalid-dsn", config.AuditBackendType("nope"))
	assert.ErrorIs(t, err, config.ErrInvalidAuditBackendType)
}

func TestNewDBMigrator_InvalidBackendType(t *testing.T) {
	mu.Lock()
	origInstance := instance
	instance = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		instance = origInstance
		mu.Unlock()
	})

	_, err := NewDBMigrator(t.Context(), "invalid-dsn", config.AuditBackendDynamoDB)
	assert.ErrorIs(t, err, config.ErrInvalidAuditBackendType)
}
