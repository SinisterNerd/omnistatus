# omniStatus Architecture

This document outlines the complete architecture and design of the omniStatus project.

## Project Structure

```
omniStatus/
├── main.go                          # Entry point
├── go.mod                           # Go module definition
├── cmd/                             # CLI command implementations
│   ├── root.go                      # Root command definition
│   ├── set.go                       # `ost set` command
│   └── clear.go                     # `ost clear` command
├── config/                          # Configuration management
│   ├── config.go                    # YAML config parser
│   └── config.example.yaml          # Example configuration
├── platform/                        # Platform integrations
│   ├── platform.go                  # Core interfaces & manager
│   ├── slack.go                     # Slack implementation
│   ├── teams.go                     # Microsoft Teams implementation
│   └── discord.go                   # Discord implementation
├── README.md                        # User documentation
├── LICENSE                          # AGPLv3 License
├── Makefile                         # Build targets
└── .gitignore                       # Git ignore rules

```

## Core Design Patterns

### 1. PresenceUpdater Interface

The heart of the extensible architecture. All platforms implement this interface:

```go
type PresenceUpdater interface {
    Name() string // unique instance name: display name + cache key
    Type() string // platform type; state mapping/Bucket key off this
    IsEnabled() bool
    UpdatePresence(ctx context.Context, update PresenceUpdate) error
    ClearPresence(ctx context.Context) error
}
```

**Benefits:**
- Add new platforms without modifying existing code
- Each platform can implement its own API communication
- Type-safe platform implementations

**Multiple instances:** Slack and GitHub can run several accounts. The
top-level `slack:`/`github:` config blocks keep the names "slack"/"github";
extra accounts live under `instances:` (each with a `type`) and are built by
`platform.NewInstanceUpdaters`. Anything keyed by *name* (cache, `--platform`,
display, icon/color config via `Config.BlockFor`) is per-instance; anything
about *behavior* (`Bucket`, `platformStates` in the menubar/tray apps) must use
`Type()`. Teams/Discord are single-instance. Not yet verified live with real
second-account tokens (as of 2026-10-07).

### 2. Manager Pattern

The `Manager` struct coordinates updates across all platforms:

```go
type Manager struct {
    updaters map[string]PresenceUpdater
}
```

**Key Methods:**
- `Register(updater PresenceUpdater)` - Add a new platform
- `UpdateAll(ctx, update)` - Update all enabled platforms concurrently
- `ClearAll(ctx)` - Clear all enabled platforms concurrently

**Concurrency Strategy:**
- Uses goroutines to update platforms simultaneously
- Each update runs in its own goroutine
- Errors are collected and reported without blocking other updates
- Context with timeout prevents indefinite hangs

### 3. Configuration Management

YAML-based configuration at `~/.config/omnistatus/config.yaml`:

```go
type Config struct {
    Slack   *PlatformConfig
    Teams   *PlatformConfig
    Discord *PlatformConfig
}

type PlatformConfig struct {
    Enabled bool
    Token   string
    Extra   map[string]string  // Platform-specific settings
}
```

**Features:**
- Single configuration file for all platforms
- File permissions set to 0600 for security
- Graceful error handling for missing config

## Component Details

### config/config.go

**Responsibilities:**
- Load YAML configuration from `~/.config/omnistatus/config.yaml`
- Parse into Go structs using `gopkg.in/yaml.v3`
- Provide SaveConfig for future enhancements
- Handle directory creation with proper permissions

**Key Functions:**
- `LoadConfig()` - Read and parse config file
- `SaveConfig(cfg)` - Write config to file
- `getConfigPath()` - Resolve config file location

### platform/platform.go

**Responsibilities:**
- Define `PresenceUpdater` interface
- Define `PresenceUpdate` struct
- Implement `Manager` for coordinating updates
- Handle concurrent execution with goroutines

**Key Types:**
- `PresenceState` - Enum for active/away/dnd
- `PresenceUpdate` - Struct containing status/emoji/state
- `PresenceUpdater` - Interface for platform implementations
- `Manager` - Coordinates all platform updates

### platform/slack.go

**Responsibilities:**
- Implement Slack API integration
- Update user profile status and emoji
- Set user presence (active/away)

**Slack API Used:**
- `users.profile.set` - Set status message and emoji
- `users.setPresence` - Set presence status

**Implementation Details:**
- HTTP POST requests with Bearer token authentication
- JSON payloads for API communication
- State mapping: active→active, away→away, dnd→away

**Configuration Required:**
- User OAuth Token with scopes: `users.profile:write`, `users:write`, `users:read`, `users.profile:read`
  (`users:write` is required to set presence/active-away - a separate scope from `users.profile:write`)

### platform/teams.go

**Responsibilities:**
- Implement Microsoft Graph API integration
- Set user presence and custom status message

**Graph API Used:**
- `me/presence/setPresence` - Set availability
- `me/presence` - Update status message

**Implementation Details:**
- HTTP POST/PATCH requests with Bearer token
- JSON payloads for Graph API
- State mapping: active→Available, away→Away, dnd→DoNotDisturb

**Configuration Required:**
- Microsoft Graph API access token
- Azure AD User Object ID

### platform/discord.go

**Responsibilities:**
- Implement Discord Rich Presence via local RPC socket
- Connect to Discord IPC socket
- Send presence updates without self-botting

**Discord RPC Protocol:**
- Connects to local Unix socket (or named pipe on Windows)
- Sends messages in Discord IPC format
- Frame structure: [opcode][length (4 bytes)][JSON data]

**Socket Paths:**
- macOS: `~/Library/Application Support/Discord/discord-ipc-0`
- Linux: `$XDG_RUNTIME_DIR/discord-ipc-0`
- Windows: `\\.\pipe\discord-ipc-0`

**Configuration Required:**
- Discord Application Client ID

### cmd/root.go

**Responsibilities:**
- Define root command with Cobra
- Provide main entry point for CLI

**Features:**
- Help text explaining all features
- Supports `--help` and `-h` flags

### cmd/set.go

**Responsibilities:**
- Implement `ost set` command
- Parse flags: --status, --emoji, --state
- Load config and update all platforms

**Flags:**
- `--status STRING` - Status message
- `--emoji STRING` - Emoji code
- `--state STRING` - Presence state (active/away/dnd)

**Error Handling:**
- Validates state against allowed values
- Reports any platform update failures
- Aggregates errors from concurrent updates

### cmd/clear.go

**Responsibilities:**
- Implement `ost clear` command
- Clear status across all platforms

**Behavior:**
- Resets status message to empty
- Sets presence to default state
- Works concurrently across platforms

## Data Flow

### Set Command Flow

```
main.go
  ↓
cmd.Execute()
  ↓
rootCmd.Execute()
  ↓
set.RunE() [cmd/set.go]
  ├─ config.LoadConfig()
  ├─ Create platform.Manager
  ├─ Register all updaters:
  │  ├─ platform.NewSlackUpdater()
  │  ├─ platform.NewTeamsUpdater()
  │  └─ platform.NewDiscordUpdater()
  └─ manager.UpdateAll()
      ├─ For each updater:
      │  └─ goroutine → updater.UpdatePresence()
      │     └─ HTTP POST to platform API
      ├─ Collect results in channels
      └─ Report success/error
```

### Clear Command Flow

```
main.go
  ↓
cmd.Execute()
  ↓
rootCmd.Execute()
  ↓
clear.RunE() [cmd/clear.go]
  ├─ config.LoadConfig()
  ├─ Create platform.Manager
  ├─ Register all updaters
  └─ manager.ClearAll()
      ├─ For each updater:
      │  └─ goroutine → updater.ClearPresence()
      │     └─ HTTP POST to platform API
      ├─ Collect results in channels
      └─ Report success/error
```

## Concurrency Model

The Manager uses channels to coordinate concurrent updates:

```go
errChan := make(chan error, len(m.updaters))
successChan := make(chan string, len(m.updaters))

// Launch one goroutine per platform
for name, updater := range m.updaters {
    go func() {
        // Either send error or success
        if err := updater.UpdatePresence(ctx, update) {
            errChan <- err
        } else {
            successChan <- name
        }
    }()
}

// Collect all results
for i := 0; i < len(m.updaters); i++ {
    select {
    case err := <-errChan:
        errors = append(errors, err)
    case name := <-successChan:
        successCount++
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

**Benefits:**
- All platforms update simultaneously
- No blocking on individual platform delays
- Context timeout prevents indefinite hangs
- Graceful error collection

## Adding a New Platform

1. **Create the implementation file** (`platform/newplatform.go`)

```go
type NewPlatformUpdater struct {
    enabled bool
    token   string
}

func (n *NewPlatformUpdater) Name() string {
    return "newplatform"
}

func (n *NewPlatformUpdater) IsEnabled() bool {
    return n.enabled && n.token != ""
}

func (n *NewPlatformUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
    // Implementation
}

func (n *NewPlatformUpdater) ClearPresence(ctx context.Context) error {
    // Implementation
}
```

2. **Update config/config.example.yaml**

```yaml
newplatform:
  enabled: true
  token: "YOUR_TOKEN_HERE"
  extra:
    setting1: "value1"
```

3. **Update config/config.go**

```go
type Config struct {
    // ...
    Newplatform *PlatformConfig `yaml:"newplatform"`
}
```

4. **Register in cmd/set.go and cmd/clear.go**

```go
manager.Register(platform.NewNewPlatformUpdater(cfg.Newplatform))
```

## Security Considerations

1. **Configuration File Permissions**
   - Created with 0600 (read/write owner only)
   - Prevents unauthorized access to tokens

2. **Token Storage**
   - Stored in local configuration file only
   - Never logged or printed
   - Passed directly to API requests

3. **No Self-Botting**
   - Discord integration uses local RPC socket
   - Compliant with Discord Terms of Service
   - No account automation or impersonation

4. **Error Messages**
   - Avoid exposing sensitive information in errors
   - Log API errors without revealing tokens

## Testing Recommendations

1. **Unit Tests** - Test each platform's state mapping
2. **Integration Tests** - Mock API responses
3. **Configuration Tests** - Validate YAML parsing
4. **Concurrency Tests** - Verify goroutine behavior
5. **Mock Platforms** - Stub implementations for testing

## Performance Characteristics

- **Startup Time**: ~100ms (includes config loading)
- **Update Time**: Parallel across platforms (~500ms typical for 3 platforms)
- **Memory Usage**: ~10MB runtime
- **Binary Size**: ~12MB (Go statically-linked binary)

## Future Enhancements

1. **Token Refresh** - Automatic OAuth token refresh
2. **Config Wizard** - Interactive setup command
3. **Scheduling** - Automated status changes
4. **History** - Track presence changes
5. **Shell Completion** - bash/zsh/fish support
6. **Status Presets** - Save and reuse common statuses
7. **Additional Platforms** - Lark, Mattermost, etc.
