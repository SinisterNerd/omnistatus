.PHONY: build build-menubar test clean help install lint fmt

# Build the binary
build:
	@echo "Building omniStatus..."
	go build -o ost main.go
	@echo "Build complete: ./ost"

# Build the optional macOS menu bar app (see cmd/menubar) as a minimal
# unsigned .app bundle. Not built by `make build` - it pulls in a
# CGO/Cocoa dependency (menuet) that CLI-only users don't need.
#
# A bare binary won't run: menuet's startup path unconditionally touches
# UNUserNotificationCenter, which crashes with "bundleProxyForCurrentProcess
# is nil" unless NSBundle.mainBundle resolves to a real .app structure -
# confirmed by running it as a loose binary first. LSUIElement=true hides
# it from the Dock/Cmd-Tab, matching menu-bar-only apps. No code signing -
# fine for local use; see cmd/menubar/main.go's doc comment for what's out
# of scope for v1 (notifications, auto-update, start-at-login all also
# want a signed bundle, and aren't used here anyway).
MENUBAR_APP = OmniStatus.app
build-menubar:
	@echo "Building omniStatus menu bar app..."
	@mkdir -p "$(MENUBAR_APP)/Contents/MacOS"
	go build -o "$(MENUBAR_APP)/Contents/MacOS/ost-menubar" ./cmd/menubar
	@echo '<?xml version="1.0" encoding="UTF-8"?>' > "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<plist version="1.0"><dict>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundleExecutable</key><string>ost-menubar</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundleIdentifier</key><string>com.rspence.omnistatus.menubar</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundleName</key><string>OmniStatus</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundleShortVersionString</key><string>0.1</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>CFBundlePackageType</key><string>APPL</string>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>LSUIElement</key><true/>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '<key>NSHighResolutionCapable</key><true/>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo '</dict></plist>' >> "$(MENUBAR_APP)/Contents/Info.plist"
	@echo "Build complete: ./$(MENUBAR_APP)"

# Build and launch the menu bar app in the foreground (Ctrl-C to stop).
run-menubar: build-menubar
	"./$(MENUBAR_APP)/Contents/MacOS/ost-menubar"

# Install the binary to /usr/local/bin
install: build
	@echo "Installing omniStatus to /usr/local/bin..."
	sudo mv ost /usr/local/bin/ost
	@echo "Installation complete. Use 'ost' command."

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f ost ost-menubar
	rm -rf OmniStatus.app
	go clean

# Display help
help:
	@echo "omniStatus - Makefile targets:"
	@echo "  make build          - Build the ost binary"
	@echo "  make build-menubar  - Build the optional OmniStatus.app menu bar app"
	@echo "  make run-menubar    - Build and run the menu bar app in the foreground"
	@echo "  make install        - Build and install to /usr/local/bin"
	@echo "  make test     - Run tests"
	@echo "  make fmt      - Format code"
	@echo "  make lint     - Run linter"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make help     - Show this help message"
