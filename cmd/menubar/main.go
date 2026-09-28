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

// Command omnistatus-menubar is an optional macOS menu bar front-end for
// omniStatus. It shows a live colored glyph per platform in the menu bar
// and lets you change state with a click, reusing the same platform/config
// packages as the `ost` CLI - no platform logic is duplicated here.
//
// v1 scope (see /Users/rspence/.claude/plans/swift-bubbling-gadget.md):
//   - State-only actions, no free-text "Custom Status..." dialog (use
//     `ost set --status "..."` for that).
//   - No interactive Teams device-code sign-in from the GUI - run
//     `ost set` once from a terminal first if Teams needs auth.
//   - No config hot-reload - restart to pick up config/icon edits.
//   - Runs as a bare binary, not a signed .app bundle.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/caseymrm/menuet/v2"
	"github.com/rspence/omnistatus/config"
	"github.com/rspence/omnistatus/platform"
)

const refreshInterval = 15 * time.Second

// platformEntry bundles what the menu bar needs about one enabled
// platform: its updater (for state changes) and its reader (nil if the
// platform doesn't support reading back current state - only affects
// display, writes still work).
//
// Deliberately does NOT reuse the tmux config's Icon() field: those
// values are commonly Nerd Font private-use-area glyphs (confirmed live -
// this user's config has them), which render fine in a terminal with a
// patched font but are invisible in the native menu bar, since it draws
// with the system UI font and menuet's TextRun has no font-family
// override to point at a custom font. The plain platform-initial letter
// below is guaranteed to render in any font; only the *color* (native
// menuet.Color, not tmux's #[fg=...] strings) carries the state signal.
type platformEntry struct {
	name    string
	updater platform.PresenceUpdater
	reader  platform.PresenceReader
}

// Package-level app state, set once at startup. Deliberately not
// reloaded from disk while running (see "No config hot-reload" above).
var (
	appConfig  *config.Config
	appEntries []platformEntry
	appManager *platform.Manager
)

// bucketColor maps a platform.Bucket() result to a native, dark/light-
// mode-adaptive menuet color.
var bucketColor = map[string]menuet.Color{
	"green":  menuet.SystemGreen,
	"yellow": menuet.SystemYellow,
	"red":    menuet.SystemRed,
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("omnistatus-menubar: %v", err)
	}
	appConfig = cfg
	appEntries = buildEntries(cfg)

	appManager = platform.NewManager()
	for _, e := range appEntries {
		appManager.Register(e.updater)
	}

	go pollLoop()

	menuet.App().Label = "com.rspence.omnistatus.menubar"
	menuet.App().Children = buildMenu
	menuet.App().RunApplication()
}

// buildEntries mirrors the enabled-platform filtering in cmd/status.go,
// minus Discord (paused, no reader, excluded from the menu bar entirely
// per HANDOFF.md §6) and minus any interactive Teams auth attempt (see
// package doc comment).
func buildEntries(cfg *config.Config) []platformEntry {
	all := []platform.PresenceUpdater{
		platform.NewSlackUpdater(cfg.Slack),
		platform.NewTeamsUpdater(cfg.Teams),
		platform.NewGitHubUpdater(cfg.GitHub),
	}

	var entries []platformEntry
	for _, u := range all {
		if !u.IsEnabled() {
			continue
		}
		reader, _ := u.(platform.PresenceReader)
		entries = append(entries, platformEntry{
			name:    u.Name(),
			updater: u,
			reader:  reader,
		})
	}
	return entries
}

// platformStates lists only the PresenceStates that produce a distinct,
// meaningful effect on a given platform (per the platform limitations
// documented in HANDOFF.md) rather than a generic 6-state list everywhere:
// Slack collapses dnd/busy/brb/offline into "away" server-side, and GitHub
// only has an active/busy boolean.
func platformStates(name string) []platform.PresenceState {
	switch name {
	case "slack":
		return []platform.PresenceState{platform.StateActive, platform.StateAway}
	case "teams":
		return []platform.PresenceState{
			platform.StateActive, platform.StateAway, platform.StateDND,
			platform.StateBusy, platform.StateBRB, platform.StateOffline,
		}
	case "github":
		return []platform.PresenceState{platform.StateActive, platform.StateBusy}
	}
	return nil
}

func stateLabel(state platform.PresenceState) string {
	switch state {
	case platform.StateActive:
		return "Active"
	case platform.StateAway:
		return "Away"
	case platform.StateDND:
		return "Do Not Disturb"
	case platform.StateBusy:
		return "Busy"
	case platform.StateBRB:
		return "Be Right Back"
	case platform.StateOffline:
		return "Offline"
	}
	return string(state)
}

func displayName(platformName string) string {
	return strings.ToUpper(platformName[:1]) + platformName[1:]
}

// pollLoop refreshes the menu bar title on a timer, independent of
// whether the dropdown is open, so the glyphs stay current at a glance.
func pollLoop() {
	refresh()
	ticker := time.NewTicker(refreshInterval)
	for range ticker.C {
		refresh()
	}
}

// refresh fetches current presence for every readable platform (via the
// same on-disk cache the CLI uses, so a concurrently-running `ost status`
// poller and this app don't double-hit the live APIs) and rebuilds the
// menu bar title as a run of colored glyphs, e.g. a green "S", a yellow
// "T", a red "GH". A platform that fails to respond renders gray rather
// than breaking the whole title.
func refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cacheEnabled, cacheTTL := platform.ResolveCacheSettings(appConfig)
	var cache platform.CacheFile
	cacheDirty := false
	if cacheEnabled {
		cache = platform.LoadCache()
	}

	var runs []menuet.TextRun
	for _, e := range appEntries {
		if e.reader == nil {
			continue
		}
		if len(runs) > 0 {
			runs = append(runs, menuet.TextRun{Text: "  "})
		}
		icon := strings.ToUpper(e.name[:1])
		info, _, err := platform.GetCached(ctx, e.name, e.reader, cacheEnabled, cacheTTL, &cache, &cacheDirty)
		if err != nil {
			runs = append(runs, menuet.TextRun{Text: icon, Color: menuet.Gray})
			continue
		}
		color := bucketColor[platform.Bucket(e.name, info.Availability)]
		runs = append(runs, menuet.TextRun{Text: icon, Color: color})
	}

	if cacheEnabled && cacheDirty {
		_ = platform.SaveCache(cache)
	}

	title := ""
	if len(runs) == 0 {
		title = "omniStatus"
	}
	menuet.App().SetMenuState(&menuet.MenuState{Title: title, Runs: runs})
}

// buildMenu is menuet's Children callback - it's called fresh every time
// the dropdown opens, so it always reflects current state (subject to the
// same cache TTL as the title).
func buildMenu() []menuet.MenuItem {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var items []menuet.MenuItem
	for _, e := range appEntries {
		items = append(items, platformMenuItems(ctx, e)...)
	}

	items = append(items, menuet.Separator{})
	for _, state := range []platform.PresenceState{
		platform.StateActive, platform.StateAway, platform.StateDND,
		platform.StateBusy, platform.StateBRB, platform.StateOffline,
	} {
		s := state
		items = append(items, menuet.Regular{
			Text: "Set All → " + stateLabel(s),
			Clicked: func() {
				setAll(platform.PresenceUpdate{State: s})
			},
		})
	}
	items = append(items, menuet.Regular{
		Text:    "Clear All",
		Clicked: clearAll,
	})

	return items
}

// platformMenuItems builds one platform's section of the dropdown: a
// non-clickable "Name: currentstate" label, then its meaningful state
// actions, then a separator. Teams shows a sign-in prompt instead of
// state actions if interactive auth hasn't been completed via the CLI yet.
func platformMenuItems(ctx context.Context, e platformEntry) []menuet.MenuItem {
	name := displayName(e.name)

	if teamsUpdater, ok := e.updater.(*platform.TeamsUpdater); ok && teamsUpdater.NeedsInteractiveAuth() {
		return []menuet.MenuItem{
			menuet.Regular{Text: fmt.Sprintf("%s: sign-in required (run “ost set” once)", name), Color: menuet.LabelSecondary},
			menuet.Separator{},
		}
	}

	label := name
	if e.reader != nil {
		cacheEnabled, cacheTTL := platform.ResolveCacheSettings(appConfig)
		var cache platform.CacheFile
		cacheDirty := false
		if cacheEnabled {
			cache = platform.LoadCache()
		}
		if info, _, err := platform.GetCached(ctx, e.name, e.reader, cacheEnabled, cacheTTL, &cache, &cacheDirty); err == nil {
			label = fmt.Sprintf("%s: %s", name, info.Availability)
		}
	}

	items := []menuet.MenuItem{
		menuet.Regular{Text: label, Color: menuet.LabelSecondary},
	}
	for _, state := range platformStates(e.name) {
		s := state
		updater := e.updater
		items = append(items, menuet.Regular{
			Text: stateLabel(s),
			Clicked: func() {
				setOne(updater, platform.PresenceUpdate{State: s})
			},
		})
	}
	items = append(items, menuet.Separator{})

	return items
}

func setOne(updater platform.PresenceUpdater, update platform.PresenceUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := updater.UpdatePresence(ctx, update); err != nil {
		log.Printf("omnistatus-menubar: %s: %v", updater.Name(), err)
	}
	refresh()
}

func setAll(update platform.PresenceUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := appManager.UpdateAll(ctx, update); err != nil {
		log.Printf("omnistatus-menubar: set all: %v", err)
	}
	refresh()
}

func clearAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := appManager.ClearAll(ctx); err != nil {
		log.Printf("omnistatus-menubar: clear all: %v", err)
	}
	refresh()
}
