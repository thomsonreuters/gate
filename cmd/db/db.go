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

// Package db provides CLI commands for managing database operations,
// including running and rolling back schema migrations.
package db

import "github.com/spf13/cobra"

// DBCmd is the root Cobra command for database-related subcommands.
// It groups subcommands such as migrate and displays help when invoked without a subcommand.
var DBCmd = &cobra.Command{
	Use:          "db",
	Short:        "Database commands",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	DBCmd.AddCommand(migrateCmd)
}
