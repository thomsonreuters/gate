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
	"log/slog"

	"github.com/thomsonreuters/gate/internal/sts/audit"
)

// ConsoleLogger logs audit entries as structured slog output.
type ConsoleLogger struct {
	logger *slog.Logger
}

// NewConsoleLogger returns a console audit logger that writes to the given slog.Logger.
func NewConsoleLogger(logger *slog.Logger) *ConsoleLogger {
	return &ConsoleLogger{logger: logger}
}

// Log writes the audit entry to stderr in JSON format.
func (l *ConsoleLogger) Log(ctx context.Context, entry *audit.AuditEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	l.logger.LogAttrs(ctx, slog.LevelInfo, "audit", slog.Any("entry", entry))
	return nil
}

// Close is a no-op for the console logger.
func (l *ConsoleLogger) Close() error {
	return nil
}
