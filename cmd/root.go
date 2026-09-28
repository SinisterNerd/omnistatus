// omniStatus
// Copyright (C) 2024 omniStatus Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"fmt"
	"os"

	"github.com/SinisterNerd/omnistatus/config"
	"github.com/spf13/cobra"
)

// configPathFlag holds the value of the global --config flag, if provided.
var configPathFlag string

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "ost",
	Short: "omniStatus - unified presence orchestrator",
	Long: `omniStatus (ost) is a lightweight CLI tool for updating your presence
and availability across multiple communication platforms simultaneously.

Configure your API tokens and platform settings in ~/.config/omnistatus/config.yaml
(or pass --config to use a config file at a custom location, e.g. a portable
config on a USB drive or shared server).

Supported platforms:
  • Slack
  • Microsoft Teams
  • Discord (via local Rich Presence)

Usage:
  ost set --status "Working" --emoji ":computer:" --state active
  ost clear
  ost --config ./my-config.yaml status`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if configPathFlag != "" {
			config.SetConfigPath(configPathFlag)
		}
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&configPathFlag, "config", "", "Path to config file (default: ~/.config/omnistatus/config.yaml)")
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
