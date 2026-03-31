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

// Package db provides database connections and migrations for the Gate service.
// It includes a DynamoDB client singleton (see dynamo.go) and PostgreSQL/GORM connection
// and migration helpers (see sql.go). Both use package-level singletons protected by mutexes.
package db

import (
	"context"
	"errors"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	migratePostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/db/migrations"
	"github.com/thomsonreuters/gate/internal/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	// MigrateSource is the name passed to migrate.NewWithInstance for the migration source (iofs).
	MigrateSource = "iofs"
	// Postgres is the database driver name used for migrations and the embedded migration directory.
	Postgres = "postgres"
)

var (
	// ErrMigrationSource is returned when the embedded migration source cannot be read.
	ErrMigrationSource = errors.New("failed to read migration source")
	// ErrUnderlyingDB is returned when the underlying *sql.DB cannot be extracted from the GORM DB.
	ErrUnderlyingDB = errors.New("failed to extract underlying sql.DB")
	// ErrMigrateDriver is returned when the PostgreSQL migrate driver cannot be created.
	ErrMigrateDriver = errors.New("failed to create migrate driver")

	// Singleton GORM instance and mutex for NewDB/NewDBMigrator.
	instance *gorm.DB
	mu       sync.Mutex
)

// NewDB returns a singleton instance of the database connection.
func NewDB(ctx context.Context, dsn string, databaseType config.AuditBackendType) (*gorm.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		sqlDB, err := instance.DB()
		if err == nil && sqlDB.Ping() == nil {
			return instance, nil
		}
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		instance = nil
	}

	var (
		db  *gorm.DB
		err error
	)

	gormConfig := &gorm.Config{
		Logger: logger.NewGormLogger(logger.WithIgnoreRecordNotFoundError()),
	}

	switch databaseType {
	case config.AuditBackendSQL:
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	case config.AuditBackendDynamoDB:
		return nil, config.ErrInvalidAuditBackendType
	default:
		return nil, config.ErrInvalidAuditBackendType
	}

	if err != nil {
		return nil, err
	}

	db = db.WithContext(ctx)
	instance = db

	return instance, nil
}

// NewDBMigrator returns a migrate instance for the given database.
func NewDBMigrator(ctx context.Context, dsn string, databaseType config.AuditBackendType) (*migrate.Migrate, error) {
	db, err := NewDB(ctx, dsn, databaseType)
	if err != nil {
		return nil, err
	}

	switch databaseType {
	case config.AuditBackendSQL:
		source, err := iofs.New(migrations.PostgresFS, Postgres)
		if err != nil {
			return nil, errors.Join(ErrMigrationSource, err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, errors.Join(ErrUnderlyingDB, err)
		}

		driver, err := migratePostgres.WithInstance(sqlDB, &migratePostgres.Config{})
		if err != nil {
			return nil, errors.Join(ErrMigrateDriver, err)
		}

		return migrate.NewWithInstance(MigrateSource, source, Postgres, driver)
	case config.AuditBackendDynamoDB:
		return nil, config.ErrInvalidAuditBackendType
	default:
		return nil, config.ErrInvalidAuditBackendType
	}
}
