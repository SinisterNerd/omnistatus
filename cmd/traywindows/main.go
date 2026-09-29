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

//go:build windows

// Command omnistatus-tray is an optional Windows system tray front-end for
// omniStatus, the Windows equivalent of cmd/menubar (macOS). Reuses
// platform/config directly - no platform logic duplicated here, same
// pattern as the CLI and the macOS app.
//
// Design differences from the macOS app, both forced by real platform
// constraints (see docs/dev/HANDOFF.md §10.3 for the fuller writeup):
//   - Windows tray icons are icon-only - systray.SetTitle is a documented
//     no-op on Windows (confirmed by reading the library source before
//     writing this). So instead of per-platform colored letters in the
//     bar (the macOS approach), this renders a single dot reflecting the
//     *worst* status across all enabled platforms (red > yellow > green),
//     with the full per-platform breakdown in the tooltip (hover) and the
//     dropdown menu (click) - both of which Windows tray icons do
//     support natively.
//   - fyne.io/systray's Go API is imperative (build menu items once,
//     mutate them later, listen on a per-item channel for clicks) rather
//     than menuet's declarative "rebuild the menu each time it opens"
//     model, so the structure here looks different even though the
//     underlying logic (buildEntries, platformStates, stateLabel) is
//     copied near-verbatim from cmd/menubar.
//
// v1 scope, same as the macOS app: state-only actions, no free-text
// "Custom Status..." dialog (use `ost set --status "..."` for that), and
// no interactive Teams device-code sign-in from the GUI - run `ost set`
// once from a terminal first if Teams needs auth.
package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"strings"
	"time"

	"fyne.io/systray"
	"github.com/SinisterNerd/omnistatus/config"
	"github.com/SinisterNerd/omnistatus/platform"
)

const refreshInterval = 15 * time.Second

// platformEntry bundles what the tray app needs about one enabled
// platform: its updater (for state changes) and its reader (nil if the
// platform doesn't support reading back current state - only affects
// display, writes still work). Same shape as cmd/menubar's.
type platformEntry struct {
	name    string
	updater platform.PresenceUpdater
	reader  platform.PresenceReader
}

// platformMenuUI holds the live systray.MenuItem for one platform's
// top-level menu entry, whose title gets updated on every refresh to show
// current state (e.g. "Slack (away)") - systray's persistent-object model
// means these are built once and mutated, not rebuilt.
type platformMenuUI struct {
	name    string
	topItem *systray.MenuItem
}

var (
	appConfig  *config.Config
	appEntries []platformEntry
	appManager *platform.Manager
	appUIs     []platformMenuUI
)

// bucketColor maps a platform.Bucket() result to the dot color. errorColor
// is used when every enabled platform failed to respond (a single flaky
// platform doesn't affect the icon - see refresh()).
var (
	bucketColor = map[string]color.RGBA{
		"green":  {R: 34, G: 197, B: 94, A: 255},
		"yellow": {R: 234, G: 179, B: 8, A: 255},
		"red":    {R: 239, G: 68, B: 68, A: 255},
	}
	errorColor = color.RGBA{R: 128, G: 128, B: 128, A: 255}
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("omnistatus-tray: %v", err)
	}
	appConfig = cfg
	appEntries = buildEntries(cfg)

	appManager = platform.NewManager()
	for _, e := range appEntries {
		appManager.Register(e.updater)
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTooltip("omniStatus")
	buildMenu()
	refresh()
	go pollLoop()
}

func onExit() {}

// buildEntries mirrors the enabled-platform filtering in cmd/status.go and
// cmd/menubar - minus Discord (paused, no reader, excluded entirely per
// HANDOFF.md §4.4/§10.3) and minus any interactive Teams auth attempt (see
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
// documented in HANDOFF.md), same as cmd/menubar: Slack collapses
// dnd/busy/brb/offline into "away" server-side, GitHub only has an
// active/busy boolean.
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

// buildMenu constructs every menu item once at startup and wires their
// click channels. Unlike cmd/menubar's declarative rebuild-on-open model,
// systray's items are persistent objects - refresh() mutates their titles
// later rather than recreating them.
func buildMenu() {
	for _, e := range appEntries {
		topItem := systray.AddMenuItem(displayName(e.name), "")
		appUIs = append(appUIs, platformMenuUI{name: e.name, topItem: topItem})

		if teamsUpdater, ok := e.updater.(*platform.TeamsUpdater); ok && teamsUpdater.NeedsInteractiveAuth() {
			signIn := topItem.AddSubMenuItem("Sign-in required (run “ost set” once)", "")
			signIn.Disable()
			continue
		}

		updater := e.updater
		for _, state := range platformStates(e.name) {
			s := state
			item := topItem.AddSubMenuItem(stateLabel(s), "")
			go func() {
				for range item.ClickedCh {
					setOne(updater, platform.PresenceUpdate{State: s})
				}
			}()
		}
	}

	systray.AddSeparator()
	for _, state := range []platform.PresenceState{
		platform.StateActive, platform.StateAway, platform.StateDND,
		platform.StateBusy, platform.StateBRB, platform.StateOffline,
	} {
		s := state
		item := systray.AddMenuItem("Set All → "+stateLabel(s), "")
		go func() {
			for range item.ClickedCh {
				setAll(platform.PresenceUpdate{State: s})
			}
		}()
	}

	clearItem := systray.AddMenuItem("Clear All", "")
	go func() {
		for range clearItem.ClickedCh {
			clearAll()
		}
	}()

	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "")
	go func() {
		<-quitItem.ClickedCh
		systray.Quit()
	}()
}

// pollLoop refreshes the tray icon/tooltip/menu titles on a timer,
// independent of whether the menu is open, so the icon stays current at a
// glance.
func pollLoop() {
	ticker := time.NewTicker(refreshInterval)
	for range ticker.C {
		refresh()
	}
}

// refresh fetches current presence for every readable platform (via the
// same on-disk cache the CLI and macOS app use, so several callers
// polling around the same time don't each hit the live APIs separately),
// updates each platform's menu-item title, and sets the tray icon to the
// worst (most attention-worthy) bucket across all of them - red beats
// yellow beats green. A platform that errors doesn't affect the icon
// color (same "don't let one flaky platform break the display" approach
// as the tmux "?" fallback and the macOS app's gray glyph), but is noted
// in its menu title and the tooltip. If every platform errors, the icon
// falls back to gray.
func refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cacheEnabled, cacheTTL := platform.ResolveCacheSettings(appConfig)
	var cache platform.CacheFile
	cacheDirty := false
	if cacheEnabled {
		cache = platform.LoadCache()
	}

	severity := map[string]int{"green": 0, "yellow": 1, "red": 2}
	worst := ""
	var tooltipLines []string
	anySuccess := false

	for i, e := range appEntries {
		if e.reader == nil {
			continue
		}
		info, _, err := platform.GetCached(ctx, e.name, e.reader, cacheEnabled, cacheTTL, &cache, &cacheDirty)
		if err != nil {
			appUIs[i].topItem.SetTitle(displayName(e.name) + ": error")
			tooltipLines = append(tooltipLines, displayName(e.name)+": error")
			continue
		}

		anySuccess = true
		appUIs[i].topItem.SetTitle(fmt.Sprintf("%s (%s)", displayName(e.name), info.Availability))
		tooltipLines = append(tooltipLines, fmt.Sprintf("%s: %s", displayName(e.name), info.Availability))

		bucket := platform.Bucket(e.name, info.Availability)
		if worst == "" || severity[bucket] > severity[worst] {
			worst = bucket
		}
	}

	if cacheEnabled && cacheDirty {
		_ = platform.SaveCache(cache)
	}

	tooltip := "omniStatus"
	if len(tooltipLines) > 0 {
		tooltip = "omniStatus\n" + strings.Join(tooltipLines, "\n")
	}
	systray.SetTooltip(tooltip)

	dotColor := errorColor
	if anySuccess {
		dotColor = bucketColor[worst]
	}
	if icon, err := renderDotIcon(dotColor); err == nil {
		systray.SetIcon(icon)
	} else {
		log.Printf("omnistatus-tray: failed to render icon: %v", err)
	}
}

func setOne(updater platform.PresenceUpdater, update platform.PresenceUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := updater.UpdatePresence(ctx, update); err != nil {
		log.Printf("omnistatus-tray: %s: %v", updater.Name(), err)
	}
	refresh()
}

func setAll(update platform.PresenceUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := appManager.UpdateAll(ctx, update); err != nil {
		log.Printf("omnistatus-tray: set all: %v", err)
	}
	refresh()
}

func clearAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := appManager.ClearAll(ctx); err != nil {
		log.Printf("omnistatus-tray: clear all: %v", err)
	}
	refresh()
}
