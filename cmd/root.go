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

// Package cmd provides the Gate CLI entrypoint and root command.
// It wires config loading, subcommands (server, db), and runs the Cobra root.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thomsonreuters/gate/cmd/db"
	"github.com/thomsonreuters/gate/cmd/server"
	"github.com/thomsonreuters/gate/internal/config"
	"github.com/thomsonreuters/gate/internal/constants"
	"github.com/thomsonreuters/gate/internal/logger"
)

var (
	// ConfigPath is the path to the configuration file, set via --config flag.
	ConfigPath string
)

var rootCmd = &cobra.Command{
	Use:          "gate",
	Short:        "GATE: GitHub Authenticated Token Exchange",
	Long:         "A secure token service for exchanging OIDC tokens for repository-scoped installation tokens.",
	Version:      fmt.Sprintf("%s (commit: %s, built: %s)", constants.Version, constants.CommitHash, constants.BuildTimestamp),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		logger.InitDefaultLevel()
		if _, err := config.Load(cmd.Context(), ConfigPath); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command and parses the CLI.
// On error it exits the process with code 1.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfigPath, "config", "c", "", "Path to config file")

	rootCmd.AddCommand(server.ServerCmd)
	rootCmd.AddCommand(db.DBCmd)
}
