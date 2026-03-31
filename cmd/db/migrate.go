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
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/db"
)

// ErrConfigNotLoaded is returned when the application configuration
// has not been initialised before running a migration command.
var ErrConfigNotLoaded = errors.New("config not loaded")

// ErrMigrationNotSupported is returned when the configured database
// backend does not support schema migrations.
var ErrMigrationNotSupported = errors.New("migrations not supported for configured database type")

// ErrSQLConfigRequired is returned when a migration command is
// invoked but no SQL configuration is present.
var ErrSQLConfigRequired = errors.New("sql config is required for migrations")

// migrateVersion holds the target migration version supplied via --version flag (0 means all).
var migrateVersion uint

var migrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Run database migrations",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var migrateUpCmd = &cobra.Command{
	Use:          "up",
	Short:        "Apply migrations (all or up to a specific version)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		m, err := newDBMigrator(ctx)
		if err != nil {
			if errors.Is(err, ErrMigrationNotSupported) {
				slog.InfoContext(ctx, "Migrations not supported for configured backend, skipping")
				return nil
			}
			return err
		}

		defer func() { _, _ = m.Close() }()

		if migrateVersion > 0 {
			slog.InfoContext(ctx, "Migrating up to version", "version", migrateVersion)
			err = m.Migrate(migrateVersion)
		} else {
			slog.InfoContext(ctx, "Applying all pending migrations")
			err = m.Up()
		}

		if errors.Is(err, migrate.ErrNoChange) {
			slog.InfoContext(ctx, "No new migrations to apply")
			return nil
		}
		if err != nil {
			return handleMigrateError(err, "up")
		}

		slog.InfoContext(ctx, "Migrations applied successfully")
		printVersion(ctx, m)
		return nil
	},
}

var migrateDownCmd = &cobra.Command{
	Use:          "down",
	Short:        "Rollback migrations (all or down to a specific version)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		m, err := newDBMigrator(ctx)
		if err != nil {
			return err
		}
		defer func() { _, _ = m.Close() }()

		if migrateVersion > 0 {
			slog.InfoContext(ctx, "Migrating down to version", "version", migrateVersion)
			err = m.Migrate(migrateVersion)
		} else {
			slog.InfoContext(ctx, "Rolling back all migrations")
			err = m.Down()
		}

		if errors.Is(err, migrate.ErrNoChange) {
			slog.InfoContext(ctx, "No migrations to rollback")
			return nil
		}
		if err != nil {
			return handleMigrateError(err, "down")
		}

		slog.InfoContext(ctx, "Rollback completed successfully")
		printVersion(ctx, m)
		return nil
	},
}

var migrateVersionCmd = &cobra.Command{
	Use:          "version",
	Short:        "Print the current migration version",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		m, err := newDBMigrator(ctx)
		if err != nil {
			return err
		}
		defer func() { _, _ = m.Close() }()

		printVersion(ctx, m)
		return nil
	},
}

var migrateForceCmd = &cobra.Command{
	Use:          "force",
	Short:        "Force set a migration version (use to fix a dirty state)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		m, err := newDBMigrator(ctx)
		if err != nil {
			return err
		}
		defer func() { _, _ = m.Close() }()

		slog.InfoContext(ctx, "Forcing migration version", "version", migrateVersion)
		if err := m.Force(int(migrateVersion)); err != nil {
			return fmt.Errorf("force version failed: %w", err)
		}

		slog.InfoContext(ctx, "Version forced successfully")
		printVersion(ctx, m)
		return nil
	},
}

// newDBMigrator builds a migrate.Migrate instance from the current config.
// It returns ErrConfigNotLoaded, ErrMigrationNotSupported, or
// ErrSQLConfigRequired when the config is missing or invalid.
func newDBMigrator(ctx context.Context) (*migrate.Migrate, error) {
	cfg := config.GetCurrent()
	if cfg == nil {
		return nil, ErrConfigNotLoaded
	}

	if !cfg.Audit.IsMigrationSupported() {
		return nil, ErrMigrationNotSupported
	}

	if cfg.Audit.SQL == nil {
		return nil, ErrSQLConfigRequired
	}

	return db.NewDBMigrator(ctx, cfg.Audit.SQL.DSN, cfg.Audit.Backend)
}

// printVersion logs the current migration version, or "no migrations applied" / an error.
func printVersion(ctx context.Context, m *migrate.Migrate) {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		slog.InfoContext(ctx, "No migrations applied yet")
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get version", "error", err)
		return
	}
	slog.InfoContext(ctx, "Current migration version", "version", version, "dirty", dirty)
}

// handleMigrateError returns a user-actionable message for dirty/locked states,
// or wraps the original error with the migration direction for context.
// Callers must check ErrNoChange before calling this function.
func handleMigrateError(err error, direction string) error {
	var dirtyErr migrate.ErrDirty
	if errors.As(err, &dirtyErr) {
		return fmt.Errorf(
			"database is in a dirty state at version %d; "+
				"inspect the failed migration then run 'db migrate force --version %d' to reset",
			dirtyErr.Version, dirtyErr.Version,
		)
	}

	if errors.Is(err, migrate.ErrLocked) || errors.Is(err, migrate.ErrLockTimeout) {
		return fmt.Errorf("database is locked by another migration process: %w", err)
	}

	return fmt.Errorf("migration %s failed: %w", direction, err)
}

func init() {
	migrateCmd.PersistentFlags().UintVar(&migrateVersion, "version", 0, "Target migration version (0 = all)")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
	migrateCmd.AddCommand(migrateForceCmd)
}
