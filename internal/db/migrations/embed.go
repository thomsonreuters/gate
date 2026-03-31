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

// Package migrations embeds the database migration files for PostgreSQL.
package migrations

import "embed"

// PostgresFS is the embedded filesystem containing postgres/*.sql migration files.
// Used by the migrate library as the migration source.
//
//go:embed postgres/*.sql
var PostgresFS embed.FS
