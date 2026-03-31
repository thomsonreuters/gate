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
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/testutil"
)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func TestConsoleLogger_Compiles(t *testing.T) {
	t.Parallel()
	var _ audit.AuditEntryBackend = (*ConsoleLogger)(nil)
}

func TestConsoleLogger_Log_Granted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewConsoleLogger(newTestLogger(&buf))

	entry := testutil.ValidGrantedEntry()
	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	assert.Equal(t, "INFO", record["level"])
	assert.Equal(t, "audit", record["msg"])

	e, ok := record["entry"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, entry.RequestID, e["request_id"])
	assert.Equal(t, entry.Caller, e["caller"])
	assert.Equal(t, entry.TargetRepository, e["target_repository"])
	assert.Equal(t, entry.PolicyName, e["policy_name"])
	assert.Equal(t, string(entry.Outcome), e["outcome"])
	assert.Equal(t, entry.TokenHash, e["token_hash"])
	assert.Equal(t, entry.GitHubClientID, e["github_client_id"])
}

func TestConsoleLogger_Log_Denied(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewConsoleLogger(newTestLogger(&buf))

	entry := testutil.ValidDeniedEntry()
	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	e, ok := record["entry"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, string(audit.OutcomeDenied), e["outcome"])
	assert.Equal(t, entry.DenyReason, e["deny_reason"])
}

func TestConsoleLogger_Log_ValidationError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewConsoleLogger(newTestLogger(&buf))

	err := logger.Log(t.Context(), &audit.AuditEntry{})
	require.ErrorIs(t, err, audit.ErrInvalidRequestID)
	assert.Empty(t, buf.String())
}

func TestConsoleLogger_Log_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewConsoleLogger(newTestLogger(&buf))

	entry := testutil.ValidGrantedEntry()
	entry.Claims = nil
	entry.Permissions = nil

	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	e, ok := record["entry"].(map[string]any)
	require.True(t, ok)
	_, hasClaims := e["claims"]
	_, hasPermissions := e["permissions"]
	assert.False(t, hasClaims)
	assert.False(t, hasPermissions)
}

func TestConsoleLogger_Close(t *testing.T) {
	t.Parallel()

	logger := NewConsoleLogger(slog.Default())
	require.NoError(t, logger.Close())
}
