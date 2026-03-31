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

package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domain "github.com/thomsonreuters/gate/internal/sts/audit"
	"github.com/thomsonreuters/gate/internal/sts/audit/backends"
	"github.com/thomsonreuters/gate/internal/testutil"
)

const sqlTable = "audit_logs"

func newSQLLogger(t *testing.T) (*backends.SQLLogger, *testutil.PostgresContainer) {
	t.Helper()
	pg := testutil.NewPostgresContainer(t)
	testutil.RunPostgresMigrations(t, pg.DB)
	return backends.NewSQLLogger(pg.DB), pg
}

func TestSQL_LogGranted(t *testing.T) {
	t.Parallel()
	logger, pg := newSQLLogger(t)

	entry := grantedEntry("sql-granted-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	var count int64
	err := pg.DB.Table(sqlTable).Where("request_id = ?", entry.RequestID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestSQL_LogDenied(t *testing.T) {
	t.Parallel()
	logger, pg := newSQLLogger(t)

	entry := deniedEntry("sql-denied-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	var got domain.AuditEntry
	err := pg.DB.Table(sqlTable).Where("request_id = ?", entry.RequestID).First(&got).Error
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeDenied, got.Outcome)
	assert.Equal(t, "no matching policy rule", got.DenyReason)
}

func TestSQL_UniqueConstraint(t *testing.T) {
	t.Parallel()
	logger, _ := newSQLLogger(t)

	entry := grantedEntry("sql-dup-001")
	require.NoError(t, logger.Log(t.Context(), entry))

	duplicate := *entry
	require.Error(t, logger.Log(t.Context(), &duplicate))
}

func TestSQL_JSONBRoundtrip(t *testing.T) {
	t.Parallel()
	logger, pg := newSQLLogger(t)

	entry := grantedEntry("sql-jsonb-001")
	entry.Claims = map[string]string{
		"sub":        "repo:example-org/example-repo:ref:refs/heads/main",
		"repository": "example-org/example-repo",
	}
	entry.Permissions = map[string]string{
		"contents": "write",
		"packages": "read",
		"metadata": "read",
	}
	require.NoError(t, logger.Log(t.Context(), entry))

	var got domain.AuditEntry
	err := pg.DB.Table(sqlTable).Where("request_id = ?", entry.RequestID).First(&got).Error
	require.NoError(t, err)
	assert.Equal(t, entry.Permissions, got.Permissions)
	assert.Equal(t, entry.Claims, got.Claims)
}

func TestSQL_Close(t *testing.T) {
	t.Parallel()
	logger, _ := newSQLLogger(t)
	require.NoError(t, logger.Close())
}
