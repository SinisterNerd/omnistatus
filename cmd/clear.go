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
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/SinisterNerd/omnistatus/config"
)

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear your presence status across all configured platforms",
	Long: `Clear your presence status across all enabled platforms.

This command resets your status message and availability to defaults
simultaneously across all enabled platforms (Slack, Teams, Discord).

Example:
  ost clear`,
	RunE: runClear,
}

var clearPlatformFlag []string

func init() {
	clearCmd.Flags().StringSliceVar(&clearPlatformFlag, "platform", nil, "Only clear these platform instances (names as shown by 'ost status'; repeat or comma-separate). Default: all enabled")

	RootCmd.AddCommand(clearCmd)
}

// runClear executes the clear command
func runClear(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create platform manager (optionally limited by --platform)
	manager, err := buildManager(cfg, clearPlatformFlag)
	if err != nil {
		return err
	}

	// Clear all enabled platforms concurrently
	// Discord's local RPC handshake can occasionally take 15-20+ seconds
	// to respond (observed directly), so give real headroom beyond that.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := manager.ClearAll(ctx); err != nil {
		return fmt.Errorf("failed to clear presence: %w", err)
	}

	fmt.Println("\n✓ Presence cleared successfully")
	return nil
}
