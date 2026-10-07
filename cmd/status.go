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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SinisterNerd/omnistatus/config"
	"github.com/SinisterNerd/omnistatus/platform"
	"github.com/spf13/cobra"
)

var (
	statusPlatformFlag string
	statusFormatFlag   string
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get current presence status from configured platforms",
	Long: `Retrieve the current presence/availability from enabled platforms
that support reading status back (currently: Teams).

Useful for displaying your status in a tmux status bar or other scripts.

Note: this command will NOT trigger an interactive sign-in even if Teams
credentials are missing/expired, since it's meant to be called frequently
(e.g., every few seconds from tmux) - it will just report an error in
that case. Run "ost set" once to complete sign-in.

Caching: if configured (see the "cache" section in config.yaml), results
are cached locally for a short TTL to avoid redundant API calls when
multiple callers (e.g. several tmux panes) poll status around the same
time. Disabled by default. Example config:

  cache:
    enabled: true
    ttl: "10s"

Cached results are marked with "(cached)" in default output format.

tmux format: a single horizontal line with one colored glyph per enabled,
readable platform - ideal for "set -g status-right '#(ost status --format
tmux)'". Each platform renders as "#[fg=COLOR]ICON#[default]", where ICON
and the three availability colors (green/yellow/red) are configurable per
platform via a "tmux:" block in that platform's config entry, e.g.:

  slack:
    enabled: true
    token: "xoxp-..."
    tmux:
      icon: ""              # any string: a letter, a word, a Nerd Font glyph
      color_green: "green"  # active/available
      color_yellow: "yellow" # away/transitional
      color_red: "red"      # busy/dnd/offline

Any field left out falls back to a sensible default (platform initial as
the icon; plain "green"/"yellow"/"red" as the colors). Colors accept
anything tmux's #[fg=...] understands: a name, "colourNNN", or (tmux 2.9+
with a truecolor terminal) a hex value like "#ff8800". A platform that
fails to respond is rendered as a dim "?" rather than breaking the line.

By default each platform shows as just its icon. Set "display:
tmux_show_state: true" (a top-level config section, not per-platform) to
also append the raw state text after each icon, e.g. "S Away" instead of
just "S".

json format: a single JSON object keyed by platform name, e.g.
'{"slack": {"availability": "away", "cached": false}}'. Includes
"activity" and "expires_at" (RFC3339) when the platform reports them,
"cached": true when the value came from the local cache, and an "error"
field instead of the other fields if that platform failed to respond.
The command's exit code is still non-zero if any platform errored, even
though the JSON itself is always valid - check the "error" fields for
which platform(s) failed rather than relying on exit code alone.

Multiple accounts: Slack and GitHub can have extra accounts under a
top-level "instances:" map (each with a "type" of slack or github); the
instance name is what shows up here and in --platform. See
config.example.yaml.

Examples:
  ost status                                    # Show all readable platforms
  ost status --platform teams                   # Show only Teams
  ost status --platform teams --format short    # Just the raw value (for scripts)
  ost status --format tmux                      # Single line for tmux status-right
  ost status --format json                      # Machine-readable, e.g. for jq`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusPlatformFlag, "platform", "", "Only show status for this platform (e.g., teams)")
	statusCmd.Flags().StringVar(&statusFormatFlag, "format", "full", "Output format: full (name + availability + activity), short (just availability), tmux (single colored horizontal line), or json (machine-readable)")

	RootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	type readerEntry struct {
		name     string
		typ      string
		reader   platform.PresenceReader
		cfgBlock *config.PlatformConfig
	}

	// Build every updater, then keep only the ones that are enabled AND
	// implement PresenceReader. Teams, Slack, and GitHub support reading;
	// Discord does not (RPC has no "get current activity" command), so
	// it's automatically excluded here without any special-casing.
	//
	// NOTE: Teams uses existing tokens only - this command deliberately
	// does NOT trigger interactive device-code sign-in, since it may be
	// called frequently (e.g., from a tmux status bar refresh).
	allUpdaters := []platform.PresenceUpdater{
		platform.NewSlackUpdater(cfg.Slack),
		platform.NewTeamsUpdater(cfg.Teams),
		platform.NewDiscordUpdater(cfg.Discord),
		platform.NewGitHubUpdater(cfg.GitHub),
	}
	instanceUpdaters, err := platform.NewInstanceUpdaters(cfg)
	if err != nil {
		return err
	}
	allUpdaters = append(allUpdaters, instanceUpdaters...)

	var readers []readerEntry
	for _, updater := range allUpdaters {
		if !updater.IsEnabled() {
			continue
		}
		reader, ok := updater.(platform.PresenceReader)
		if !ok {
			continue
		}
		readers = append(readers, readerEntry{name: updater.Name(), typ: updater.Type(), reader: reader, cfgBlock: cfg.BlockFor(updater.Name())})
	}

	if statusPlatformFlag != "" {
		var filtered []readerEntry
		for _, r := range readers {
			if r.name == statusPlatformFlag {
				filtered = append(filtered, r)
			}
		}
		readers = filtered
	}

	if len(readers) == 0 {
		if statusPlatformFlag != "" {
			return fmt.Errorf("platform '%s' is not enabled or does not support status reading", statusPlatformFlag)
		}
		return fmt.Errorf("no platforms with status reading support are enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Caching avoids redundant API calls when multiple callers (e.g.,
	// several tmux panes/sessions each polling their status bar every
	// few seconds) query status around the same time. This is purely a
	// local read-cache, colocated with the active config file, and never
	// affects `ost set`/`ost clear`.
	cacheEnabled, cacheTTL := platform.ResolveCacheSettings(cfg)
	var cache platform.CacheFile
	cacheDirty := false
	if cacheEnabled {
		cache = platform.LoadCache()
	}

	// tmux format renders every platform as a single glyph on one line, so
	// it's built up in a buffer and printed once at the end rather than
	// line-by-line like the other formats - a partial/interleaved line
	// would be useless in a tmux status bar.
	var tmuxLine strings.Builder

	// json format is likewise accumulated and printed once at the end - a
	// single valid JSON document, not one JSON blob per line.
	jsonResult := make(map[string]statusJSONEntry)

	var firstErr error
	for _, r := range readers {
		info, fromCache, err := platform.GetCached(ctx, r.name, r.reader, cacheEnabled, cacheTTL, &cache, &cacheDirty)
		if err != nil {
			switch statusFormatFlag {
			case "tmux":
				// Don't let one flaky platform break the whole status
				// line - render it as a dim "?" and keep going.
				if tmuxLine.Len() > 0 {
					tmuxLine.WriteByte(' ')
				}
				tmuxLine.WriteString("#[fg=colour238]?#[default]")
			case "json":
				jsonResult[r.name] = statusJSONEntry{Error: err.Error()}
			default:
				fmt.Printf("%s: error: %v\n", r.name, err)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		cacheSuffix := ""
		if fromCache {
			cacheSuffix = " (cached)"
		}

		switch statusFormatFlag {
		case "short":
			fmt.Println(info.Availability)
		case "tmux":
			bucket := platform.Bucket(r.typ, info.Availability)
			icon := r.cfgBlock.Icon(platform.DefaultIcon(r.name))
			color := r.cfgBlock.ColorFor(bucket)
			label := icon
			if cfg.TmuxShowStateOrDefault() {
				label = icon + " " + info.Availability
			}
			if tmuxLine.Len() > 0 {
				tmuxLine.WriteByte(' ')
			}
			fmt.Fprintf(&tmuxLine, "#[fg=%s]%s#[default]", color, label)
		case "json":
			jsonResult[r.name] = statusJSONEntry{
				Availability: info.Availability,
				Activity:     info.Activity,
				ExpiresAt:    info.ExpiresAt,
				Cached:       fromCache,
			}
		default:
			if info.ExpiresAt != nil {
				fmt.Printf("%s: %s (%s) - expires %s%s\n", r.name, info.Availability, info.Activity, info.ExpiresAt.Local().Format("15:04:05"), cacheSuffix)
			} else {
				fmt.Printf("%s: %s (%s)%s\n", r.name, info.Availability, info.Activity, cacheSuffix)
			}
		}
	}

	if statusFormatFlag == "tmux" {
		fmt.Println(tmuxLine.String())
	}

	if statusFormatFlag == "json" {
		encoded, err := json.MarshalIndent(jsonResult, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
		fmt.Println(string(encoded))
	}

	if cacheEnabled && cacheDirty {
		// Best-effort: a failure to persist the cache shouldn't fail the
		// command, since the status was already successfully retrieved
		// and printed above.
		_ = platform.SaveCache(cache)
	}

	return firstErr
}

// statusJSONEntry is one platform's entry in `ost status --format json`'s
// output map (keyed by platform name). Either Availability is populated
// (success) or Error is (failure) - never both.
type statusJSONEntry struct {
	Availability string     `json:"availability,omitempty"`
	Activity     string     `json:"activity,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Cached       bool       `json:"cached"`
	Error        string     `json:"error,omitempty"`
}
