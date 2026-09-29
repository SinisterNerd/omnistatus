# omniStatus (ost)

[![Release](https://img.shields.io/github/v/release/SinisterNerd/omnistatus)](https://github.com/SinisterNerd/omnistatus/releases/latest)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A lightweight, unified status and presence orchestrator for terminal-centric developers. Update your availability across Slack, Microsoft Teams, and GitHub simultaneously with a single CLI command.

## Features

- **Unified Presence Management**: Update status, emoji, and availability state across multiple platforms with one command
- **Concurrent Updates**: Leverages Go goroutines for fast, simultaneous platform updates
- **Minimal Configuration**: Simple YAML configuration at `~/.config/omnistatus/config.yaml`
- **Rich Presence Support**: Integrates with Discord via local RPC socket (no self-botting)
- **Extensible Architecture**: Generic interface design makes adding new platforms straightforward
- **tmux-Friendly**: Designed for binding to keyboard shortcuts in terminal multiplexers and displaying status on the status line

## Installation

### From Source

```bash
git clone https://github.com/SinisterNerd/omnistatus.git
cd omnistatus
go build -o ost main.go
sudo mv ost /usr/local/bin/
```

### From Binary

Pre-built CLI binaries for macOS (amd64/arm64), Linux (amd64/arm64), and Windows (amd64/arm64) are attached to each [GitHub Release](https://github.com/SinisterNerd/omnistatus/releases). Download the one matching your platform, then on macOS/Linux:

```bash
chmod +x ost-<os>-<arch>
sudo mv ost-<os>-<arch> /usr/local/bin/ost
```

Note: only the CLI is cross-platform. The macOS menu bar app (`cmd/menubar`) depends on Cocoa/CGO and must be built locally on a Mac via `make build-menubar` - it isn't part of the release binaries.

## macOS Menu Bar App (Optional)

An optional native menu bar front-end lives alongside the CLI in `cmd/menubar` — build it with `make build-menubar` (produces `OmniStatus.app`), or `make run-menubar` to build and launch it directly. It shows a live colored glyph per enabled platform (green/yellow/red, reflecting the same state logic as `ost status`) and lets you change state with a click, no terminal needed. See `docs/dev/HANDOFF.md` §5.5 for the full design/limitations (v1 has no free-text status dialog — use `ost set --status` for that — and no interactive Teams sign-in from the GUI, so run `ost set` once first if Teams needs auth).

## Windows Tray App (Optional)

The Windows equivalent lives in `cmd/traywindows` — build with `make build-tray-windows` (cross-compiles both amd64 and arm64, e.g. Surface devices with Snapdragon chips, into `dist/`). Same idea as the macOS app, click the tray icon for a menu of quick state changes, but the tray icon itself shows a single colored dot reflecting the *worst* status across your enabled platforms (Windows tray icons can't show inline colored text the way the macOS menu bar can). Two ways to see the full per-platform breakdown: hover the icon for a tooltip, or click **Show Status Window** in the menu for a small borderless always-on-top panel that stays visible without needing to keep hovering — drag it anywhere on the desktop (click and hold anywhere on the panel, no title bar needed), and dismiss it via the menu item again. That panel does its own text rendering, so unlike the tray icon or the macOS app, it can honor your configured `tmux.icon`/`tmux.color_*` per platform, plus a Windows-only `tmux.font` field to point at an installed font (e.g. a Nerd Font) so the real glyph renders instead of a plain letter. A few more display options, all optional and global (not per-platform) - see the `display:` section in [`config/config.example.yaml`](config/config.example.yaml): `tray_layout` (`vertical`, the default, or `horizontal`), and `tray_show_state`/`tmux_show_state` to toggle whether each platform's raw state text (e.g. "Away") shows alongside its icon. Same v1 scope as the macOS app otherwise: state-only actions (no free-text status dialog), and no interactive Teams sign-in from the GUI.

**Tested live on Windows on ARM, working well.** Real bugs found and fixed along the way, all confirmed: a missing config file caused a completely silent failure (fixed with a proper error dialog), and the floating window was fixed-position with a distracting "busy" cursor on hover (fixed: it's now draggable, and the cursor was a one-line missing-cursor bug in the window class). See `docs/dev/HANDOFF.md` §5.9 for the full story.

## Quick Start

### 1. Configure Platforms

Create the configuration file:

```bash
mkdir -p ~/.config/omnistatus
cp config/config.example.yaml ~/.config/omnistatus/config.yaml
```

Edit `~/.config/omnistatus/config.yaml` and add your API tokens:

```yaml
slack:
  enabled: true
  token: "xoxp-YOUR_SLACK_TOKEN"

teams:
  enabled: true
  extra:
    client_id: "YOUR_AZURE_APP_CLIENT_ID"
    tenant_id: "YOUR_AZURE_TENANT_ID"
  # token/refresh_token are filled in automatically on first run via a
  # one-time device-code sign-in - see the Microsoft Teams section below

github:
  enabled: true
  token: "YOUR_GITHUB_PERSONAL_ACCESS_TOKEN"
```

See [`config/config.example.yaml`](config/config.example.yaml) for the fully annotated reference, including optional per-platform tmux status-line settings and response caching (covered under [Configuration](#configuration) below).

### 2. Update Your Status

```bash
# Set status with emoji and state
ost set --status "In a meeting" --emoji ":calendar:" --state away

# Set status as active
ost set --status "Working on feature X" --emoji ":computer:" --state active

# Do Not Disturb mode
ost set --status "Deep work" --emoji ":brain:" --state dnd

# Clear status
ost clear
```

### 3. Bind to tmux (Optional)

Add to your `~/.tmux.conf`:

```tmux
bind-key -n M-a run-shell "ost set --status 'In tmux' --emoji ':computer:' --state active"
bind-key -n M-w run-shell 'ost clear'
```

Double quotes around the whole `run-shell` command, single quotes for the individual values - not the other way around. tmux's own config-file parser (separate from your shell) mishandles nested double quotes inside a single-quoted `run-shell` argument. See [`docs/TMUX.md`](docs/TMUX.md) for more tmux examples.

## Platform Capabilities

Not every platform supports both concepts omniStatus deals with:

- **State** — an availability/presence indicator (e.g., Available/Away/Busy)
- **Status** — free-form text + emoji shown alongside your name

| Platform | `--state` supported | `--status`/`--emoji` supported | `--duration` supported |
|----------|:---:|:---:|:---:|
| Slack | Yes (collapses to active/away - see [Slack section](#slack)) | Yes | No |
| Microsoft Teams | Yes (6 states) | Yes (text only, no emoji field) | Availability only - not the status message (see [Microsoft Teams section](#microsoft-teams)) |
| Discord | Yes *(paused, see note below)* | Yes *(paused, see note below)* | No |
| GitHub | No - GitHub has no availability concept, `--state` only toggles the "Busy" flag | Yes - this is the *only* signal GitHub has, so omitting `--status`/`--emoji` means nothing will change | Yes (`expiresAt`) |

`--duration` accepts Go-style duration strings (e.g. `30m`, `1h30m`) and auto-clears the status after that period elapses - the platform itself handles the expiration, not a background process on your machine.

> **A note on Discord:** the Discord integration is implemented and technically correct (verified directly against Discord's RPC protocol - handshake, nonce, and command all succeed with no errors), but Discord Rich Presence display is gated behind client-side settings that vary between Discord versions/accounts and haven't been reliably reproducible for testing. Because of this volatility, and because Discord isn't a "business" tool for most users of this project, further Discord polish is **paused** rather than actively developed. The code path remains in place and may work as-is for some users/setups - it's simply not being chased further right now. This may be revisited in a future development phase.

### `--state` Support By Platform

Not every `--state` value produces a distinct effect on every platform - some platforms have fewer real states than the CLI does, so several values collapse into the same result:

| State | Slack | Microsoft Teams | GitHub |
|---|:---:|:---:|:---:|
| `active` | ✅ Active | ✅ Available | ✅ Clears the "Busy" flag |
| `away` | ✅ Away | ✅ Away | ⚠️ Sets "Busy" flag (no distinct "away") |
| `dnd` | ⚠️ Collapses to Away | ✅ Do Not Disturb | ⚠️ Sets "Busy" flag |
| `busy` | ⚠️ Collapses to Away | ✅ Busy | ⚠️ Sets "Busy" flag |
| `brb` | ⚠️ Collapses to Away | ✅ Be Right Back | ⚠️ Sets "Busy" flag |
| `offline` | ⚠️ Collapses to Away | ✅ Offline | ⚠️ Sets "Busy" flag |

Slack only has two real presence states (active/away), so `dnd`/`busy`/`brb`/`offline` all map to "away" server-side - not a bug, just what Slack's API exposes. GitHub only has a single "Busy" boolean (`limitedAvailability`), so any non-`active` state sets it `true` with no finer granularity - see the [GitHub section](#github) below for the caveat about needing `--status`/`--emoji` text for it to actually take effect. Microsoft Teams is the only platform where all six states are genuinely distinct.

## Usage

### Commands

#### `ost set`

Updates presence across all enabled platforms.

**Flags:**
- `--status STRING` - Status message to display
- `--emoji STRING` - Emoji code (e.g., `:coffee:`, `:calendar:`)
- `--state STRING` - Presence state: `active`, `away`, `dnd`, `busy`, `brb`, or `offline` (default: `active`)
- `--duration STRING` - Optional: auto-clear after this long (e.g. `30m`, `1h30m`). Only Teams and GitHub honor this - see [Platform Capabilities](#platform-capabilities)
- `--config STRING` - Optional: path to a custom config file (default: `~/.config/omnistatus/config.yaml`)

**Examples:**
```bash
ost set --status "Coffee break" --emoji ":coffee:" --state away
ost set --status "In meeting"
ost set --status "Deep work" --state dnd --emoji ":brain:"
ost set --status "Heads down" --state busy --duration 2h
```

#### `ost clear`

Clears presence status across all enabled platforms and resets to default state.

**Example:**
```bash
ost clear
```

## Configuration

### File Location
`~/.config/omnistatus/config.yaml`

### File Structure

```yaml
slack:
  enabled: true|false       # Enable/disable Slack integration
  token: "string"           # Slack User OAuth Token
  tmux:                     # Optional: `ost status --format tmux` display (see below)
    icon: "S"
    color_green: "green"
    color_yellow: "yellow"
    color_red: "red"

teams:
  enabled: true|false       # Enable/disable Teams integration
  token: "string"           # Access token (auto-filled/refreshed - don't set by hand)
  extra:
    client_id: "string"     # Azure app registration Client ID
    tenant_id: "string"     # Azure tenant ID
    refresh_token: "string" # Auto-filled after first sign-in - don't set by hand
  tmux:
    icon: "T"
    color_green: "green"
    color_yellow: "yellow"
    color_red: "red"

github:
  enabled: true|false       # Enable/disable GitHub integration
  token: "string"           # Personal Access Token (classic), `user` scope
  tmux:
    icon: "G"
    color_green: "green"
    color_yellow: "yellow"
    color_red: "red"

# Optional: cache `ost status`/tmux/menu bar reads locally for a short TTL,
# so several callers polling around the same time (multiple tmux panes, the
# menu bar app, etc.) don't each hit the live APIs separately. Never
# affects `ost set`/`ost clear`, which always make live requests. Disabled
# by default.
cache:
  enabled: true|false
  ttl: "10s"                # Go duration string, e.g. "10s", "30s", "1m"
```

Every `tmux:` block is optional and every field within it is optional - anything left out falls back to a sensible default (the platform's initial letter as the icon, plain `"green"`/`"yellow"`/`"red"` as the colors). `icon` accepts any string - a letter, a word, or a Nerd Font glyph if your terminal font has one. Colors accept anything tmux's `#[fg=...]` understands: a color name, a 256-color index (`"colour208"`), or (tmux 2.9+ with a truecolor terminal) a hex value (`"#ff8800"`). See `ost status --help` for the full tmux-format writeup, and the [macOS Menu Bar App](#macos-menu-bar-app-optional) section above - the menu bar app reuses `icon`'s *concept* but always renders plain letters natively, since native macOS menus can't render arbitrary custom/Nerd Font glyphs the way a terminal can.

## Platform Setup

### Slack

1. Go to [Slack API Apps](https://api.slack.com/apps)
2. Create a new app
3. Add User Token Scopes:
   - `users.profile:write` - Set status text/emoji
   - `users:write` - Set presence (active/away) - **required**, easy to miss since it's separate from `users.profile:write`
   - `users:read` - Read presence via `ost status`
   - `users.profile:read` - (optional) read status text/emoji via `ost status`
4. Generate a User OAuth Token
5. Add the token to your config file

### Microsoft Teams

Uses delegated OAuth 2.0 via the device code flow - no client secret, no local redirect server, works fine over SSH.

1. Register an app in [Azure Portal](https://portal.azure.com)
2. **Authentication** → Add a platform → **Mobile and desktop applications** (this makes it a public client - no client secret needed or accepted)
3. **API permissions** → Add a permission → Microsoft Graph → **Delegated permissions** (not Application) → add `Presence.ReadWrite` and `User.Read` → Grant admin consent
4. Add `client_id` and `tenant_id` (both from the app registration's Overview page) to your config file under `teams.extra`
5. Run any `ost set` command - since `token`/`refresh_token` are missing, omniStatus automatically starts a one-time device-code sign-in: it prints a URL and a short code, you complete sign-in on any device with a browser, and the tokens are saved back to your config file automatically from then on (including silent refresh - access tokens expire ~1hr, the refresh token handles renewal transparently)

**`--status` support:** `ost set --status "..."` sets a real custom status message on Teams via Microsoft Graph's `presence/setStatusMessage` action - no extra setup or permissions needed beyond the `Presence.ReadWrite` scope above. Two caveats: there's no separate emoji field (plain text only, so `--emoji` is a no-op for Teams), and `--duration` doesn't apply to the status message specifically - Graph accepts but silently ignores the expiration on this endpoint, so it only auto-clears availability, not the status text (`ost clear` clears both immediately, unlike waiting out a duration).

### Discord *(paused - see [Platform Capabilities](#platform-capabilities))*

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Copy the Client ID
4. Add the Client ID to your config file
5. Ensure Discord desktop app is running

**Note:** omniStatus uses Discord Rich Presence via local IPC socket, not self-botting. This is ToS-compliant.

**Known limitation:** the underlying RPC implementation is correct and Discord's protocol acknowledges commands successfully, but whether the Rich Presence actually *displays* depends on client-side settings (e.g., Activity Status privacy toggles) that vary across Discord versions and haven't been reliably reproducible. This integration is considered experimental/paused - functional at the protocol level, but not guaranteed to visibly update your Discord status. Revisiting this is a candidate for a future development phase rather than current priority.

### GitHub

GitHub has no availability/presence concept - only a profile status (the emoji + text shown under your avatar), optionally with an expiration and a "Busy" flag.

1. Go to [GitHub Personal Access Tokens](https://github.com/settings/tokens)
2. Generate a new token (classic) with the `user` scope
3. Add the token to your config file

**Note:** Since `--state` has no direct GitHub equivalent, it's repurposed to control the "Busy" (`limitedAvailability`) flag - any state other than `active` sets it to `true`. GitHub's API silently ignores the whole update (including the Busy flag) if both emoji and message are empty, so `ost set --state busy` with no `--status`/`--emoji` automatically falls back to a default `:no_entry:` emoji to make sure the flag actually sticks; any emoji/message you do supply is used as-is.

**`--duration` support:** GitHub natively supports auto-expiring statuses via `expiresAt`, same as Teams. `ost set --status "Heads down" --state busy --duration 2h` will automatically clear after 2 hours - GitHub handles this server-side, not omniStatus.

## Architecture

### Core Components

- **`config/config.go`** - YAML configuration loader
- **`platform/platform.go`** - Generic `PresenceUpdater` interface and manager
- **`platform/slack.go`** - Slack HTTP API integration
- **`platform/teams.go`** - Microsoft Graph API integration
- **`platform/discord.go`** - Discord Rich Presence (RPC socket)
- **`cmd/root.go`** - Root Cobra command
- **`cmd/set.go`** - Set command implementation
- **`cmd/clear.go`** - Clear command implementation

### Design Patterns

**Extensible Interface:**
All platforms implement the `PresenceUpdater` interface, making it easy to add new platforms:

```go
type PresenceUpdater interface {
    Name() string
    IsEnabled() bool
    UpdatePresence(ctx context.Context, update PresenceUpdate) error
    ClearPresence(ctx context.Context) error
}
```

**Concurrent Updates:**
The `Manager` uses goroutines to update all platforms simultaneously for minimal latency:

```go
go func(name string, updater PresenceUpdater) {
    updater.UpdatePresence(ctx, update)
}(name, updater)
```

## Adding New Platforms

1. Create a new file: `platform/newplatform.go`
2. Implement the `PresenceUpdater` interface
3. Register in `cmd/set.go` and `cmd/clear.go`:

```go
manager.Register(platform.NewNewPlatformUpdater(cfg.Newplatform))
```

4. Add configuration to `config/config.example.yaml`

## Troubleshooting

### "Config file not found"
Ensure `~/.config/omnistatus/config.yaml` exists and contains valid YAML.

### "Discord socket not found. Is Discord running?"
Make sure the Discord desktop app is running. omniStatus requires local Discord to be active.

### "Slack API error: invalid_auth"
Verify your Slack User Token is valid and hasn't expired. Check the token in [Slack App Settings](https://api.slack.com/apps).

### "Graph API returned status 401"
Your Microsoft Graph API token has expired. Re-authenticate to obtain a fresh token.

## Contributing

Contributions are welcome! See [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) for the process and our (currently unenforced, but appreciated) [Developer Certificate of Origin](https://developercertificate.org/) sign-off convention.

## License

GNU Affero General Public License v3.0 (AGPLv3) - see LICENSE file for details

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [YAML v3](https://github.com/go-yaml/yaml) for configuration management

## Roadmap

- [x] Binary releases for macOS, Linux, Windows - see [GitHub Releases](https://github.com/SinisterNerd/omnistatus/releases)
- [x] **Windows status/tray app** (`cmd/traywindows`) - equivalent of the macOS menu bar app. Pure Go, no CGO (confirmed). Build with `make build-tray-windows`. **Not yet verified on real Windows** - builds clean cross-compiled from macOS, but untested at runtime; see `docs/dev/HANDOFF.md` §10.3 for details and what to check first.
- [ ] **Web service / JSON API** - self-hosted HTTP layer over `platform`/`config` (same reuse pattern as the CLI and menu bar apps), enabling clients this project won't build natively itself: a browser-based PWA for iOS/Android ("Add to Home Screen", no App Store), home automation (Home Assistant, etc.), or anything else that can make an HTTP request. Planned as a **separate repo**, not part of this one - see `docs/dev/HANDOFF.md` §10 for why, plus the licensing/hosting considerations for a possible future paid tier.
- [ ] Native iOS/iPadOS app (Swift/SwiftUI) and native Android app (Kotlin), consuming the web API above
- [ ] Linux status/tray tool - **on hold pending demand**. The Linux desktop tray ecosystem is meaningfully more fragmented than Windows/macOS (no OS-level tray API, GNOME dropped native tray support entirely, DBus/StatusNotifierItem is the closest thing to a standard, some window managers have no tray concept at all). In the meantime, `ost status --format json` already gives Linux users with tiling WMs/custom bars a way to build their own display.
- [ ] Configuration wizard (`ost config init`)
- [ ] Revisit Discord Rich Presence display reliability (currently paused - protocol implementation works, client-side display is inconsistent)
- [ ] Lark integration
- [ ] Mattermost integration
- [ ] Status scheduling/automation
- [ ] Presence status history
- [ ] Shell completion (bash, zsh, fish)
