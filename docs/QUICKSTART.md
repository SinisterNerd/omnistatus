# omniStatus - Quick Start Guide

## ✅ What's Been Generated

A complete, production-ready Go project with:

- **9 Go source files** (~1,500 lines of idiomatic Go code)
- **3 comprehensive documentation files** (README, ARCHITECTURE, DEVELOPMENT)
- **3 platforms fully implemented** (Slack, Teams, Discord)
- **CLI ready to use** (set, clear commands with all flags)
- **Build system** (Makefile with convenient targets)
- **Binary compiled** and ready to run (12MB arm64 executable)

## 🚀 Get Started in 60 Seconds

### 1. Set Up Configuration

```bash
# Create config directory
mkdir -p ~/.config/omnistatus

# Copy example config
cp config/config.example.yaml ~/.config/omnistatus/config.yaml

# Edit config with your API tokens
nano ~/.config/omnistatus/config.yaml
```

### 2. Add Your API Tokens

For **Slack**:
1. Go to https://api.slack.com/apps
2. Create app → Add scopes: `users.profile:write`, `users:write`, `users:read`, `users.profile:read`
   (`users:write` is required to set active/away - it's separate from `users.profile:write`!)
3. Generate User OAuth Token
4. Paste in config

For **Teams**:
1. Register app in https://portal.azure.com
2. Get Graph API token via OAuth 2.0
3. Add your Azure AD Object ID
4. Paste both in config

For **Discord**:
1. Go to https://discord.com/developers/applications
2. Create app and copy Client ID
3. Paste in config
4. Ensure Discord desktop is running

### 3. Try It Out

```bash
# Update status
./ost set --status "Coffee break" --emoji ":coffee:" --state away

# Check it worked on Slack, Teams, or Discord!

# Clear status
./ost clear
```

### 4. (Optional) Install System-Wide

```bash
make install
# Now use 'ost' from anywhere
```

### 5. (Optional) Bind to tmux

Add to `~/.tmux.conf`:
```tmux
bind-key -n M-a run-shell 'ost set --status "In tmux" --emoji ":terminal:" --state active'
bind-key -n M-w run-shell 'ost clear'
```

## 📚 Documentation

| Document | Purpose |
|----------|---------|
| **README.md** | User guide, features, troubleshooting |
| **dev/ARCHITECTURE.md** | Technical design, interfaces, concurrency |
| **dev/DEVELOPMENT.md** | Contributor guide, adding platforms |
| **dev/FILES_SUMMARY.md** | Complete file inventory |

## 🔧 Available Commands

```bash
./ost --help                          # Show help
./ost set --help                      # Set command help
./ost clear --help                    # Clear command help

./ost set --status "Working"                           # Minimal
./ost set --status "In meeting" --emoji ":calendar:"   # With emoji
./ost set --status "Focus" --state dnd                 # With state
./ost set --status "DND" --emoji ":brain:" --state dnd # Full

./ost clear                           # Clear status
```

## 🛠️ Build Commands

```bash
make build    # Compile binary
make install  # Install to /usr/local/bin
make test     # Run tests
make fmt      # Format code
make lint     # Check code style
make clean    # Remove artifacts
make help     # Show all targets
```

## 📊 What's Implemented

### ✅ CLI Features
- [x] `ost set` command with flags
- [x] `ost clear` command
- [x] Flag validation and help
- [x] Error handling and reporting

### ✅ Configuration
- [x] YAML config parsing
- [x] Secure file handling (0600 permissions)
- [x] Per-platform settings
- [x] Example configuration

### ✅ Slack Integration
- [x] Status message updates
- [x] Emoji support
- [x] Presence state (active/away)
- [x] OAuth token authentication

### ✅ Microsoft Teams Integration
- [x] Graph API integration
- [x] Availability states (Available/Away/DoNotDisturb)
- [x] Custom status message
- [x] Azure AD authentication

### ✅ Discord Integration
- [x] Local RPC socket communication
- [x] Rich Presence updates
- [x] ToS-compliant (no self-botting)
- [x] Desktop app detection

### ✅ Architecture
- [x] PresenceUpdater interface
- [x] Platform Manager
- [x] Concurrent goroutine updates
- [x] Channel-based error collection
- [x] Context-based timeouts

### ✅ Documentation
- [x] User guide (README)
- [x] Architecture guide
- [x] Development guide
- [x] Code comments
- [x] Example configuration

## 🎯 Project Layout

```
omniStatus/
├── main.go                 # Entry point
├── cmd/                    # CLI commands
│   ├── root.go            # Base command
│   ├── set.go             # Set command
│   └── clear.go           # Clear command
├── config/                # Configuration
│   ├── config.go          # YAML parser
│   └── config.example.yaml # Example
├── platform/              # Integrations
│   ├── platform.go        # Interfaces
│   ├── slack.go           # Slack API
│   ├── teams.go           # Teams API
│   └── discord.go         # Discord RPC
├── README.md              # User guide
├── docs/                  # ARCHITECTURE.md, DEVELOPMENT.md, setup guides, etc.
│   └── dev/               # ARCHITECTURE.md, DEVELOPMENT.md, FILES_SUMMARY.md
├── Makefile               # Build
├── LICENSE                # AGPLv3
└── go.mod                 # Dependencies
```

## 🔐 Security Notes

- Config file created with 0600 permissions (owner read/write only)
- Tokens never logged or printed
- Discord uses local socket (no cloud botting)
- Slack/Teams use OAuth tokens (not passwords)

## 🐛 Troubleshooting

### "Config file not found"
```bash
cp config/config.example.yaml ~/.config/omnistatus/config.yaml
# Add your tokens
```

### "Discord socket not found"
- Ensure Discord desktop app is running
- Check socket path: `ls ~/Library/Application\ Support/Discord/`

### "Slack API error: invalid_auth"
- Verify token in config file
- Token should be User OAuth Token (starts with `xoxp-`)

### Build issues
```bash
go mod tidy      # Update dependencies
make clean       # Clean artifacts
make build       # Rebuild
```

## 📦 Next Steps

1. **Set up config** - Add your API tokens
2. **Try it out** - Test each platform
3. **Install globally** - `make install`
4. **Bind shortcuts** - Add to tmux/shell
5. **Explore docs** - Read dev/ARCHITECTURE.md for internals
6. **Contribute** - See dev/DEVELOPMENT.md for contributing

## 💡 Tips

- Use emoji codes from: https://www.webfx.com/tools/emoji-cheat-sheet/
- Test one platform at a time to debug config
- Check Discord Rich Presence in Discord settings
- Slack status updates are instant (visible to others)
- Teams changes take a moment to sync

## 📝 File Sizes

| File | Size | Purpose |
|------|------|---------|
| main.go | 94 B | Entry point |
| cmd/root.go | 1.1 KB | CLI root |
| cmd/set.go | 2.8 KB | Set command |
| cmd/clear.go | 2.2 KB | Clear command |
| config/config.go | 4.2 KB | Config parser |
| platform/platform.go | 7.0 KB | Interfaces |
| platform/slack.go | 6.8 KB | Slack API |
| platform/teams.go | 7.5 KB | Teams API |
| platform/discord.go | 9.0 KB | Discord RPC |

## ✨ Key Highlights

- **Fast**: Concurrent updates across platforms
- **Simple**: Single command, clear interface
- **Secure**: OAuth tokens, proper file permissions
- **Extensible**: Easy to add new platforms
- **Well-documented**: 3 guides + inline comments
- **Production-ready**: Error handling, validation, logging

---

**Ready to go!** Start with `./ost --help` or `make build && ./ost set --status "test"`
