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

package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SinisterNerd/omnistatus/config"
)

// PresenceState represents the user's presence state
type PresenceState string

const (
	StateActive  PresenceState = "active"
	StateAway    PresenceState = "away"
	StateDND     PresenceState = "dnd"     // Do Not Disturb
	StateBusy    PresenceState = "busy"    // Busy
	StateBRB     PresenceState = "brb"     // Be Right Back
	StateOffline PresenceState = "offline" // Appear Offline
)

// PresenceUpdate contains the status information to be updated
type PresenceUpdate struct {
	Status   string        // Status message/text
	Emoji    string        // Emoji code (e.g., ":coffee:")
	State    PresenceState // Presence state
	Duration time.Duration // Optional: how long the update should last (zero = no expiration)
}

// HasDuration reports whether an expiration duration was requested.
func (p PresenceUpdate) HasDuration() bool {
	return p.Duration > 0
}

// ExpiresAt returns the absolute expiration time for this update,
// computed from Duration relative to now. Only meaningful if HasDuration().
func (p PresenceUpdate) ExpiresAt() time.Time {
	return time.Now().Add(p.Duration)
}

// ISO8601Duration formats a time.Duration as an ISO 8601 duration string
// (e.g., "PT1H30M"), which several platform APIs (Microsoft Graph) use
// to express expiration periods. Only hours/minutes/seconds are needed
// for our use case (durations of days aren't a realistic status length).
func ISO8601Duration(d time.Duration) string {
	if d <= 0 {
		return "PT0S"
	}

	totalSeconds := int64(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	result := "PT"
	if hours > 0 {
		result += fmt.Sprintf("%dH", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dM", minutes)
	}
	if seconds > 0 || result == "PT" {
		result += fmt.Sprintf("%dS", seconds)
	}
	return result
}

// PresenceInfo represents the current presence/status read back from a platform
type PresenceInfo struct {
	Platform     string     // Platform name (e.g., "teams")
	Availability string     // Raw availability value from the platform
	Activity     string     // Raw activity value from the platform (if any)
	ExpiresAt    *time.Time // When this status will auto-clear, if the platform reports it (nil if none/not supported)
}

// PresenceReader is an optional interface platforms can implement to
// support reading back the current presence (e.g., for `ost status`).
// Not all platforms support this (e.g., Discord RPC has no read-back).
type PresenceReader interface {
	// GetPresence retrieves the current presence information
	GetPresence(ctx context.Context) (PresenceInfo, error)
}

// PresenceUpdater defines the interface that all platform implementations must satisfy
type PresenceUpdater interface {
	// Name returns the unique instance name (e.g., "slack", "teams",
	// "slack-personal"). Used as the display name and cache key.
	Name() string

	// Type returns the platform type ("slack", "teams", "discord",
	// "github"). Several instances can share one type (e.g. two Slack
	// accounts); behavior such as state mapping keys off the type.
	Type() string

	// IsEnabled returns whether this platform is enabled in the config
	IsEnabled() bool

	// UpdatePresence updates the user's presence on the platform
	// Returns an error if the update fails
	UpdatePresence(ctx context.Context, update PresenceUpdate) error

	// Clear clears the presence/status on the platform
	ClearPresence(ctx context.Context) error
}

// NewInstanceUpdaters builds updaters for the extra accounts configured
// under cfg.Instances, sorted by instance name. Only slack and github
// support multiple instances; an unknown or missing type, or a name that
// collides with a built-in platform name, is an error rather than being
// silently ignored.
func NewInstanceUpdaters(cfg *config.Config) ([]PresenceUpdater, error) {
	names := make([]string, 0, len(cfg.Instances))
	for name := range cfg.Instances {
		names = append(names, name)
	}
	sort.Strings(names)

	var updaters []PresenceUpdater
	for _, name := range names {
		block := cfg.Instances[name]
		switch name {
		case "slack", "teams", "discord", "github":
			return nil, fmt.Errorf("instances.%s: name is reserved for the top-level %s: block", name, name)
		}
		if block == nil {
			return nil, fmt.Errorf("instances.%s: empty block", name)
		}
		switch block.Type {
		case "slack":
			updaters = append(updaters, NewNamedSlackUpdater(name, block))
		case "github":
			updaters = append(updaters, NewNamedGitHubUpdater(name, block))
		default:
			return nil, fmt.Errorf("instances.%s: type must be \"slack\" or \"github\", got %q", name, block.Type)
		}
	}
	return updaters, nil
}

// Manager manages multiple presence updaters and coordinates concurrent updates
type Manager struct {
	updaters map[string]PresenceUpdater
}

// NewManager creates a new Manager instance
func NewManager() *Manager {
	return &Manager{
		updaters: make(map[string]PresenceUpdater),
	}
}

// Register adds a new PresenceUpdater to the manager
func (m *Manager) Register(updater PresenceUpdater) {
	if updater != nil {
		m.updaters[updater.Name()] = updater
	}
}

// UpdateAll updates presence across all enabled platforms concurrently using goroutines
func (m *Manager) UpdateAll(ctx context.Context, update PresenceUpdate) error {
	if len(m.updaters) == 0 {
		return fmt.Errorf("no platforms registered")
	}

	// Count only enabled platforms - disabled ones never launch a goroutine
	// and therefore never send a result, so we must not wait on them.
	enabledCount := 0
	for _, updater := range m.updaters {
		if updater.IsEnabled() {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("no platforms are enabled")
	}

	// Create channels for error collection
	errChan := make(chan error, enabledCount)
	successChan := make(chan string, enabledCount)

	// Launch concurrent updates
	for name, updater := range m.updaters {
		if !updater.IsEnabled() {
			continue
		}

		go func(name string, updater PresenceUpdater) {
			if err := updater.UpdatePresence(ctx, update); err != nil {
				errChan <- fmt.Errorf("%s: %w", name, err)
			} else {
				successChan <- name
			}
		}(name, updater)
	}

	// Collect results
	var errors []error
	successCount := 0

	for i := 0; i < enabledCount; i++ {
		select {
		case err := <-errChan:
			errors = append(errors, err)
		case name := <-successChan:
			successCount++
			fmt.Printf("✓ Updated %s\n", name)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Return combined errors if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("encountered %d error(s) during updates: %v", len(errors), errors)
	}

	return nil
}

// ClearAll clears presence across all enabled platforms concurrently using goroutines
func (m *Manager) ClearAll(ctx context.Context) error {
	if len(m.updaters) == 0 {
		return fmt.Errorf("no platforms registered")
	}

	// Count only enabled platforms - disabled ones never launch a goroutine
	// and therefore never send a result, so we must not wait on them.
	enabledCount := 0
	for _, updater := range m.updaters {
		if updater.IsEnabled() {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("no platforms are enabled")
	}

	errChan := make(chan error, enabledCount)
	successChan := make(chan string, enabledCount)

	// Launch concurrent clear operations
	for name, updater := range m.updaters {
		if !updater.IsEnabled() {
			continue
		}

		go func(name string, updater PresenceUpdater) {
			if err := updater.ClearPresence(ctx); err != nil {
				errChan <- fmt.Errorf("%s: %w", name, err)
			} else {
				successChan <- name
			}
		}(name, updater)
	}

	// Collect results
	var errors []error
	successCount := 0

	for i := 0; i < enabledCount; i++ {
		select {
		case err := <-errChan:
			errors = append(errors, err)
		case name := <-successChan:
			successCount++
			fmt.Printf("✓ Cleared %s\n", name)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Return combined errors if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("encountered %d error(s) during clear operations: %v", len(errors), errors)
	}

	return nil
}

// DefaultIcon returns the short fallback label used when no tmux icon is
// configured: the uppercased first letter of the instance name, plus the
// first letter of any "-"/"_" suffix so sibling instances stay distinct
// ("slack-work" -> "Sw", "slack-personal" -> "Sp", "teams" -> "T").
func DefaultIcon(name string) string {
	if name == "" {
		return "?"
	}
	icon := strings.ToUpper(name[:1])
	if i := strings.IndexAny(name, "-_"); i >= 0 && i+1 < len(name) {
		icon += strings.ToLower(name[i+1 : i+2])
	}
	return icon
}
