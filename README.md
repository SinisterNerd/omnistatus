# omniStatus (ost)

[![Release](https://img.shields.io/github/v/release/SinisterNerd/omnistatus)](https://github.com/SinisterNerd/omnistatus/releases/latest)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A lightweight, unified status and presence orchestrator for terminal-centric developers. Update your availability across Slack, Microsoft Teams, and Discord simultaneously with a single CLI command.

## Features

- **Unified Presence Management**: Update status, emoji, and availability state across multiple platforms with one command
- **Concurrent Updates**: Leverages Go goroutines for fast, simultaneous platform updates
- **Minimal Configuration**: Simple YAML configuration at `~/.config/omnistatus/config.yaml`
- **Rich Presence Support**: Integrates with Discord via local RPC socket (no self-botting)
- **Extensible Architecture**: Generic interface design makes adding new platforms straightforward
- **tmux-Friendly**: Designed for binding to keyboard shortcuts in terminal multiplexers

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
  token: "YOUR_GRAPH_API_TOKEN"
  extra:
    user_id: "YOUR_AZURE_AD_OBJECT_ID"

discord:
  enabled: true
  extra:
    client_id: "YOUR_DISCORD_CLIENT_ID"
```

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
bind-key -n M-a run-shell 'ost set --status "In tmux" --emoji ":terminal:" --state active'
bind-key -n M-w run-shell 'ost clear'
```

## Platform Capabilities

Not every platform supports both concepts omniStatus deals with:

- **State** — an availability/presence indicator (e.g., Available/Away/Busy)
- **Status** — free-form text + emoji shown alongside your name

| Platform | `--state` supported | `--status`/`--emoji` supported | `--duration` supported |
|----------|:---:|:---:|:---:|
| Slack | Yes (collapses to active/away - see [Slack section](#slack)) | Yes | No |
| Microsoft Teams | Yes (6 states) | No - not exposed by Microsoft Graph API | Yes (`expirationDuration`) |
| Discord | Yes *(paused, see note below)* | Yes *(paused, see note below)* | No |
| GitHub | No - GitHub has no availability concept, `--state` only toggles the "Busy" flag | Yes - this is the *only* signal GitHub has, so omitting `--status`/`--emoji` means nothing will change | Yes (`expiresAt`) |

`--duration` accepts Go-style duration strings (e.g. `30m`, `1h30m`) and auto-clears the status after that period elapses - the platform itself handles the expiration, not a background process on your machine.

> **A note on Discord:** the Discord integration is implemented and technically correct (verified directly against Discord's RPC protocol - handshake, nonce, and command all succeed with no errors), but Discord Rich Presence display is gated behind client-side settings that vary between Discord versions/accounts and haven't been reliably reproducible for testing. Because of this volatility, and because Discord isn't a "business" tool for most users of this project, further Discord polish is **paused** rather than actively developed. The code path remains in place and may work as-is for some users/setups - it's simply not being chased further right now. This may be revisited in a future development phase.

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

teams:
  enabled: true|false       # Enable/disable Teams integration
  token: "string"           # Microsoft Graph API access token
  extra:
    user_id: "string"       # Your Azure AD Object ID

discord:
  enabled: true|false       # Enable/disable Discord integration
  extra:
    client_id: "string"     # Discord application Client ID
```

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

1. Register an app in [Azure Portal](https://portal.azure.com)
2. Add API Permissions:
   - `Presence.ReadWrite`
   - `User.Read`
3. Create a client secret
4. Authenticate using OAuth 2.0 to obtain an access token
5. Get your Azure AD Object ID (User profile in Azure Portal)
6. Add token and user_id to config file

**Note:** The token obtained from client credentials flow is sufficient for presence updates. For production use, consider implementing token refresh logic.

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

**Note:** Since `--state` has no direct GitHub equivalent, it's repurposed to control the "Busy" (`limitedAvailability`) flag - any state other than `active` sets it to `true`. Because GitHub's status is *only* the emoji/message, running `ost set --state busy` with no `--status`/`--emoji` will set the Busy flag but won't display anything new, since there's no text to show.

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

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

GNU Affero General Public License v3.0 (AGPLv3) - see LICENSE file for details

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [YAML v3](https://github.com/go-yaml/yaml) for configuration management

## Roadmap

- [ ] Binary releases for macOS, Linux, Windows
- [ ] Configuration wizard (`ost config init`)
- [ ] GitHub profile status integration
- [ ] Revisit Discord Rich Presence display reliability (currently paused - protocol implementation works, client-side display is inconsistent)
- [ ] Lark integration
- [ ] Mattermost integration
- [ ] Status scheduling/automation
- [ ] Presence status history
- [ ] Shell completion (bash, zsh, fish)
