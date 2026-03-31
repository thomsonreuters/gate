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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

const (
	// defaultSlowThreshold is the default duration above which GORM queries are logged as slow.
	defaultSlowThreshold = 200 * time.Millisecond
	latencyKey           = "latency"
	rowsKey              = "rows"
	sqlKey               = "sql"
)

// GormLogger adapts slog to gorm's logger.Interface.
type GormLogger struct {
	level                     gormlogger.LogLevel
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
}

// GormLoggerOption configures a GormLogger.
type GormLoggerOption func(*GormLogger)

// WithSlowThreshold sets the duration above which queries are logged as slow.
func WithSlowThreshold(d time.Duration) GormLoggerOption {
	return func(l *GormLogger) { l.slowThreshold = d }
}

// WithIgnoreRecordNotFoundError suppresses record-not-found log entries.
func WithIgnoreRecordNotFoundError() GormLoggerOption {
	return func(l *GormLogger) { l.ignoreRecordNotFoundError = true }
}

// NewGormLogger creates a GORM logger that delegates to slog.
func NewGormLogger(opts ...GormLoggerOption) *GormLogger {
	l := &GormLogger{
		level:         gormlogger.Warn,
		slowThreshold: defaultSlowThreshold,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LogMode sets the log level for the GORM logger.
func (g *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *g
	clone.level = level
	return &clone
}

// Info logs an info-level message.
func (g *GormLogger) Info(ctx context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Info {
		slog.InfoContext(ctx, fmt.Sprintf(msg, args...))
	}
}

// Warn logs a warning-level message.
func (g *GormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Warn {
		slog.WarnContext(ctx, fmt.Sprintf(msg, args...))
	}
}

// Error logs an error-level message.
func (g *GormLogger) Error(ctx context.Context, msg string, args ...any) {
	if g.level >= gormlogger.Error {
		slog.ErrorContext(ctx, fmt.Sprintf(msg, args...))
	}
}

// Trace logs SQL execution details including duration and row count.
func (g *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []slog.Attr{
		slog.Duration(latencyKey, elapsed),
		slog.Int64(rowsKey, rows),
		slog.String(sqlKey, sql),
	}

	switch {
	case err != nil && (!errors.Is(err, gormlogger.ErrRecordNotFound) || !g.ignoreRecordNotFoundError):
		slog.LogAttrs(ctx, slog.LevelError, "query error",
			append(attrs, slog.String("error", err.Error()))...)

	case g.slowThreshold > 0 && elapsed > g.slowThreshold:
		slog.LogAttrs(ctx, slog.LevelWarn, "slow query",
			append(attrs, slog.Duration("threshold", g.slowThreshold))...)

	case g.level >= gormlogger.Info:
		slog.LogAttrs(ctx, slog.LevelDebug, "query", attrs...)
	}
}
