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

package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"
)

func setupSlogBuffer(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	return &buf
}

func parseLogEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	require.NoError(t, json.NewDecoder(buf).Decode(&entry))
	return entry
}

func TestNewGormLogger_Defaults(t *testing.T) {
	l := NewGormLogger()
	assert.Equal(t, gormlogger.Warn, l.level)
	assert.Equal(t, defaultSlowThreshold, l.slowThreshold)
	assert.False(t, l.ignoreRecordNotFoundError)
}

func TestNewGormLogger_WithOptions(t *testing.T) {
	l := NewGormLogger(
		WithSlowThreshold(500*time.Millisecond),
		WithIgnoreRecordNotFoundError(),
	)
	assert.Equal(t, 500*time.Millisecond, l.slowThreshold)
	assert.True(t, l.ignoreRecordNotFoundError)
}

func TestGormLogger_LogMode(t *testing.T) {
	l := NewGormLogger()
	l2 := l.LogMode(gormlogger.Info)

	gl, ok := l2.(*GormLogger)
	require.True(t, ok)
	assert.Equal(t, gormlogger.Info, gl.level)
	assert.Equal(t, gormlogger.Warn, l.level)
}

func TestGormLogger_Info(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Info)
	l.Info(context.Background(), "test %s", "message")

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "test message", entry["msg"])
}

func TestGormLogger_Info_Suppressed(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Warn)
	l.Info(context.Background(), "should not appear")

	assert.Empty(t, buf.String())
}

func TestGormLogger_Warn(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Warn)
	l.Warn(context.Background(), "slow %s", "query")

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "slow query", entry["msg"])
}

func TestGormLogger_Warn_Suppressed(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Error)
	l.Warn(context.Background(), "should not appear")

	assert.Empty(t, buf.String())
}

func TestGormLogger_Error(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Error)
	l.Error(context.Background(), "db %s", "error")

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, "db error", entry["msg"])
}

func TestGormLogger_Trace_Error(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Error)
	l.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, errors.New("connection refused"))

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, "query error", entry["msg"])
	assert.Equal(t, "SELECT 1", entry["sql"])
	assert.Equal(t, "connection refused", entry["error"])
}

func TestGormLogger_Trace_RecordNotFound_Ignored(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger(WithIgnoreRecordNotFoundError()).LogMode(gormlogger.Error)
	l.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, gormlogger.ErrRecordNotFound)

	assert.Empty(t, buf.String())
}

func TestGormLogger_Trace_RecordNotFound_NotIgnored(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Error)
	l.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, gormlogger.ErrRecordNotFound)

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, "query error", entry["msg"])
}

func TestGormLogger_Trace_SlowQuery(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger(WithSlowThreshold(1 * time.Millisecond)).LogMode(gormlogger.Warn)
	start := time.Now().Add(-10 * time.Millisecond)
	l.Trace(context.Background(), start, func() (string, int64) {
		return "SELECT sleep(1)", 5
	}, nil)

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "WARN", entry["level"])
	assert.Equal(t, "slow query", entry["msg"])
	assert.Equal(t, "SELECT sleep(1)", entry["sql"])
	assert.InDelta(t, float64(5), entry["rows"], 0)
}

func TestGormLogger_Trace_Normal(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Info)
	l.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 1
	}, nil)

	entry := parseLogEntry(t, buf)
	assert.Equal(t, "DEBUG", entry["level"])
	assert.Equal(t, "query", entry["msg"])
	assert.Equal(t, "SELECT 1", entry["sql"])
}

func TestGormLogger_Trace_Silent(t *testing.T) {
	buf := setupSlogBuffer(t)

	l := NewGormLogger().LogMode(gormlogger.Silent)
	l.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 1
	}, nil)

	assert.Empty(t, buf.String())
}
