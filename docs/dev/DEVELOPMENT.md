# Development Guide for omniStatus

This guide covers how to set up your development environment and contribute to omniStatus.

## Prerequisites

- Go 1.21+ ([Install Go](https://golang.org/doc/install))
- Git
- Your favorite text editor or IDE (VS Code, GoLand, vim, etc.)

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/rspence/omnistatus.git
cd omnistatus
```

### 2. Download Dependencies

```bash
go mod download
```

### 3. Build the Project

```bash
make build
# or
go build -o ost main.go
```

### 4. Verify It Works

```bash
./ost --help
./ost set --help
./ost clear --help
```

## Development Workflow

### Running Tests

```bash
# Run all tests with verbose output
make test

# Run tests for a specific package
go test -v ./cmd
go test -v ./platform
go test -v ./config
```

### Code Formatting

```bash
# Format all code
make fmt

# Or use Go's built-in formatter
go fmt ./...
```

### Linting

```bash
# Install golangci-lint (if not already installed)
brew install golangci-lint  # macOS
# or
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
make lint
```

### Building for Different Platforms

```bash
# Build for macOS (ARM64 - Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o ost-darwin-arm64

# Build for macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o ost-darwin-amd64

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o ost-linux-amd64

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o ost-windows-amd64.exe
```

## Project Structure Review

- **main.go** - Entry point
- **cmd/** - CLI commands (set, clear, root)
- **config/** - Configuration management
- **platform/** - Platform integrations and core interfaces

## Common Development Tasks

### Adding a New Platform

1. **Create the platform implementation**

```bash
touch platform/newplatform.go
```

2. **Implement the PresenceUpdater interface**

```go
package platform

type NewPlatformUpdater struct {
    enabled bool
    token   string
    // other fields...
}

func (n *NewPlatformUpdater) Name() string {
    return "newplatform"
}

func (n *NewPlatformUpdater) IsEnabled() bool {
    return n.enabled && n.token != ""
}

func (n *NewPlatformUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
    // Implementation
    return nil
}

func (n *NewPlatformUpdater) ClearPresence(ctx context.Context) error {
    // Implementation
    return nil
}
```

3. **Update configuration**

Edit `config/config.go` to add the platform config:
```go
type Config struct {
    Slack       *PlatformConfig `yaml:"slack"`
    Teams       *PlatformConfig `yaml:"teams"`
    Discord     *PlatformConfig `yaml:"discord"`
    Newplatform *PlatformConfig `yaml:"newplatform"`  // Add this
}
```

4. **Update example config**

Edit `config/config.example.yaml`:
```yaml
newplatform:
  enabled: false
  token: "YOUR_TOKEN_HERE"
  extra:
    key: "value"
```

5. **Register the platform**

In `cmd/set.go`:
```go
manager.Register(platform.NewNewPlatformUpdater(cfg.Newplatform))
```

In `cmd/clear.go`:
```go
manager.Register(platform.NewNewPlatformUpdater(cfg.Newplatform))
```

### Modifying the CLI

The CLI uses [Cobra](https://github.com/spf13/cobra). To add a new command:

1. Create a new file in `cmd/` (e.g., `cmd/status.go`)
2. Define a `cobra.Command` struct
3. Register it with the root command in `init()`

Example:
```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show current presence status",
    RunE:  runStatus,
}

func init() {
    RootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
    fmt.Println("Current status: Active")
    return nil
}
```

### Testing a Platform Integration

#### Mock Testing

```go
package platform

import (
    "context"
    "testing"
)

func TestSlackUpdater_UpdatePresence(t *testing.T) {
    updater := NewSlackUpdater(&config.PlatformConfig{
        Enabled: true,
        Token:   "test-token",
    })

    update := PresenceUpdate{
        Status: "Testing",
        Emoji:  ":test:",
        State:  StateActive,
    }

    ctx := context.Background()
    err := updater.UpdatePresence(ctx, update)
    
    // Note: This will fail without valid token and Slack API
    // For real tests, mock the HTTP client
}
```

#### With HTTP Mocking

Use a library like `httptest` to mock API responses:

```go
import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestSlackUpdater_MockedAPI(t *testing.T) {
    // Create a mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"ok":true}`))
    }))
    defer server.Close()

    // Test with mock server URL
    // ...
}
```

## Code Style Guidelines

1. **Follow Go conventions**
   - Use `gofmt` for formatting
   - Use `golangci-lint` for linting
   - Follow [Effective Go](https://golang.org/doc/effective_go)

2. **Naming conventions**
   - Use clear, descriptive names
   - Use `Updater` suffix for PresenceUpdater implementations
   - Use `Config` suffix for configuration structs

3. **Comments**
   - Add comments for exported functions
   - Explain complex logic inline
   - Use doc comments for packages

4. **Error handling**
   - Always check errors
   - Wrap errors with context using `fmt.Errorf`
   - Never ignore error returns silently

Example:
```go
// Good
file, err := os.Open("config.yaml")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}

// Bad
file, _ := os.Open("config.yaml")  // Error ignored!
```

5. **Concurrency**
   - Use goroutines carefully
   - Always use channels for communication
   - Set context timeouts for long operations

## Debugging

### Enable Verbose Output

Add debug logging to understand execution flow:

```go
import "log"

log.Printf("Updating presence on %s", updater.Name())
```

### Using a Debugger

VS Code + Go extension:

1. Add breakpoint in code
2. Run `go run main.go set --status "test"`
3. Debugger will pause at breakpoint

### Testing with Real APIs

1. Set up valid tokens in `~/.config/omnistatus/config.yaml`
2. Run `./ost set --status "test" --emoji ":test:" --state active`
3. Check the platform to verify update

## Version Management

The project uses semantic versioning. Update version tags:

```bash
git tag -a v0.1.0 -m "Initial release"
git push origin v0.1.0
```

## Documentation

When making changes, update relevant documentation:

- **Code changes** → Update code comments
- **User-facing changes** → Update README.md
- **Architecture changes** → Update ARCHITECTURE.md
- **New platforms** → Update DEVELOPMENT.md

## Performance Optimization

### Profiling

```bash
# CPU profiling
go run -cpuprofile=cpu.prof main.go set --status "test"
go tool pprof cpu.prof

# Memory profiling
go run -memprofile=mem.prof main.go set --status "test"
go tool pprof mem.prof
```

### Benchmarking

```go
func BenchmarkSlackUpdater_UpdatePresence(b *testing.B) {
    updater := NewSlackUpdater(&config.PlatformConfig{
        Enabled: true,
        Token:   "test-token",
    })
    
    update := PresenceUpdate{
        Status: "Testing",
        State:  StateActive,
    }
    
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        updater.UpdatePresence(ctx, update)
    }
}
```

Run benchmarks:
```bash
go test -bench=. ./platform
```

## Troubleshooting

### Build Errors

**Error: `package gopkg.in/yaml.v3 not found`**
```bash
go mod tidy
go mod download
```

**Error: `undefined: cobra`**
```bash
go get github.com/spf13/cobra@latest
```

### Runtime Issues

**"Config file not found"**
- Ensure `~/.config/omnistatus/config.yaml` exists
- Check file permissions: `chmod 600 ~/.config/omnistatus/config.yaml`

**"Platform is not enabled or token is missing"**
- Check `config.yaml` for correct `enabled: true`
- Verify token is set and not empty

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Add tests for new functionality
5. Run `make lint` and `make fmt`
6. Commit with clear messages: `git commit -m "Add new feature"`
7. Push to your fork: `git push origin feature/my-feature`
8. Open a Pull Request

## Release Checklist

Before releasing a new version:

- [ ] All tests passing: `make test`
- [ ] Code formatted: `make fmt`
- [ ] No linting errors: `make lint`
- [ ] README.md updated
- [ ] CHANGELOG.md created/updated
- [ ] Version bumped in code (if applicable)
- [ ] Build binaries for all platforms
- [ ] Test binaries on target platforms
- [ ] Create GitHub release with binaries
- [ ] Tag version in git

## Resources

- [Cobra Documentation](https://cobra.dev/)
- [Go Standard Library](https://golang.org/pkg/)
- [Effective Go](https://golang.org/doc/effective_go)
- [omniStatus README](README.md)
- [omniStatus Architecture](ARCHITECTURE.md)
