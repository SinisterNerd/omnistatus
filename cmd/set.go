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
	"github.com/SinisterNerd/omnistatus/platform"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set your presence status across all configured platforms",
	Long: `Set your presence status across multiple communication platforms.

This command updates your status message, emoji, and availability state
simultaneously across all enabled platforms (Slack, Teams, Discord).

Example:
  ost set --status "In a meeting" --emoji ":calendar:" --state away
  ost set --status "Coffee break" --emoji ":coffee:" --state dnd
  ost set --status "Focus time" --state busy --duration 1h30m

Not all platforms support --duration (auto-expiring status). Currently:
  Teams:   supported (Graph API expirationDuration)
  GitHub:  supported (expiresAt)
  Slack:   not supported by omniStatus yet
  Discord: paused (see README)`,
	RunE: runSet,
}

var (
	statusFlag   string
	emojiFlag    string
	stateFlag    string
	durationFlag string
)

func init() {
	setCmd.Flags().StringVar(&statusFlag, "status", "", "Status message to set")
	setCmd.Flags().StringVar(&emojiFlag, "emoji", "", "Emoji code (e.g., ':coffee:') to set")
	setCmd.Flags().StringVar(&stateFlag, "state", "active", "Presence state: active, away, dnd, busy, brb, or offline")
	setCmd.Flags().StringVar(&durationFlag, "duration", "", "Optional: auto-clear after this long (e.g. '30m', '1h30m'). Only supported by some platforms - see help text")

	RootCmd.AddCommand(setCmd)
}

// runSet executes the set command
func runSet(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate state flag
	state := platform.PresenceState(stateFlag)
	switch state {
	case platform.StateActive, platform.StateAway, platform.StateDND,
		platform.StateBusy, platform.StateBRB, platform.StateOffline:
		// Valid state
	default:
		return fmt.Errorf("invalid state '%s'. Must be one of: active, away, dnd, busy, brb, offline", stateFlag)
	}

	// Parse optional duration
	var duration time.Duration
	if durationFlag != "" {
		parsed, err := time.ParseDuration(durationFlag)
		if err != nil {
			return fmt.Errorf("invalid --duration '%s': %w (expected format like '30m', '1h30m')", durationFlag, err)
		}
		if parsed <= 0 {
			return fmt.Errorf("--duration must be positive, got '%s'", durationFlag)
		}
		duration = parsed
	}

	// Create presence update
	update := platform.PresenceUpdate{
		Status:   statusFlag,
		Emoji:    emojiFlag,
		State:    state,
		Duration: duration,
	}

	// Create platform manager and register all platforms
	manager := platform.NewManager()
	manager.Register(platform.NewSlackUpdater(cfg.Slack))

	teamsUpdater, err := getTeamsUpdater(cfg.Teams)
	if err != nil {
		return err
	}
	manager.Register(teamsUpdater)

	manager.Register(platform.NewDiscordUpdater(cfg.Discord))
	manager.Register(platform.NewGitHubUpdater(cfg.GitHub))

	instanceUpdaters, err := platform.NewInstanceUpdaters(cfg)
	if err != nil {
		return err
	}
	for _, u := range instanceUpdaters {
		manager.Register(u)
	}

	// Update all enabled platforms concurrently
	// Discord's local RPC handshake can occasionally take 15-20+ seconds
	// to respond (observed directly), so give real headroom beyond that.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := manager.UpdateAll(ctx, update); err != nil {
		return fmt.Errorf("failed to update presence: %w", err)
	}

	fmt.Println("\n✓ Presence updated successfully")
	return nil
}
