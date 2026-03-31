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
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/testutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockGormDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })

	dialector := postgres.New(postgres.Config{
		Conn:                 mockDB,
		DriverName:           "postgres",
		PreferSimpleProtocol: true,
	})

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Discard,
	})
	require.NoError(t, err)

	return db, mock
}

func expectInsertAuditLog(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestSQLLogger_Compiles(t *testing.T) {
	t.Parallel()
	var _ audit.AuditEntryBackend = (*SQLLogger)(nil)
}

func TestSQLLogger_Log_Success(t *testing.T) {
	t.Parallel()

	db, mock := newMockGormDB(t)
	logger := NewSQLLogger(db)

	entry := testutil.ValidGrantedEntry()
	expectInsertAuditLog(mock)

	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLLogger_Log_DeniedEntry(t *testing.T) {
	t.Parallel()

	db, mock := newMockGormDB(t)
	logger := NewSQLLogger(db)

	entry := testutil.ValidDeniedEntry()
	expectInsertAuditLog(mock)

	err := logger.Log(t.Context(), entry)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLLogger_Log_ValidationError(t *testing.T) {
	t.Parallel()

	db, _ := newMockGormDB(t)
	logger := NewSQLLogger(db)

	err := logger.Log(t.Context(), &audit.AuditEntry{})
	require.ErrorIs(t, err, audit.ErrInvalidRequestID)
}

func TestSQLLogger_Log_DBError(t *testing.T) {
	t.Parallel()

	db, mock := newMockGormDB(t)
	logger := NewSQLLogger(db)

	entry := testutil.ValidGrantedEntry()
	dbErr := errors.New("connection refused")

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnError(dbErr)
	mock.ExpectRollback()

	err := logger.Log(t.Context(), entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating audit log")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLLogger_Log_BeginError(t *testing.T) {
	t.Parallel()

	db, mock := newMockGormDB(t)
	logger := NewSQLLogger(db)

	entry := testutil.ValidGrantedEntry()
	mock.ExpectBegin().WillReturnError(errors.New("tx begin failed"))

	err := logger.Log(t.Context(), entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating audit log")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLLogger_Close(t *testing.T) {
	t.Parallel()

	db, mock := newMockGormDB(t)
	logger := NewSQLLogger(db)

	mock.ExpectClose()

	err := logger.Close()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
