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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/SinisterNerd/omnistatus/config"
)

// DiscordUpdater implements PresenceUpdater for Discord Rich Presence.
//
// STATUS: PAUSED / EXPERIMENTAL (as of this writing). The implementation
// below is protocol-correct - verified directly against Discord's local
// RPC server: proper HANDSHAKE, a valid nonce on every command, and
// Discord's own ACK response confirms the command was accepted with no
// errors (evt: null). Despite this, whether Rich Presence actually
// *displays* in the Discord client depends on client-side settings (e.g.
// Activity Status privacy toggles) that vary across Discord versions/
// accounts and have not been reliably reproducible during testing.
//
// Given that (a) the code is confirmed correct at the protocol level,
// (b) Discord isn't a "business" tool for most users of this project, and
// (c) further progress requires chasing undocumented client-side behavior
// rather than fixing bugs in our own code, work on Discord display
// reliability is paused rather than actively pursued. It may already work
// for some users/setups as-is. Revisiting this (e.g., adding a
// self-test/diagnostic command, or investigating specific client
// versions) is a candidate for a future development phase.
//
// NOTE: DiscordUpdater intentionally does NOT implement PresenceReader
// (no GetPresence method), so it won't show up in `ost status`. The
// Discord RPC protocol has no "get current activity" command - SET_ACTIVITY
// is push-only/write-only, and there's no supported way to read back what
// was last set.
//
// IMPORTANT LIMITATION: this only sets Rich Presence (the activity text
// shown under your name, e.g. "Busy - Coffee break"). It does NOT and
// CANNOT change your actual Discord status indicator (the online/idle/
// dnd/invisible dot) - that setting is not exposed via the local RPC API
// to third-party apps at all. Only the user manually changes that, or a
// self-bot using the full Gateway API would be needed, which we
// deliberately avoid since it violates Discord's Terms of Service.
type DiscordUpdater struct {
	enabled  bool
	clientID string // Discord app client ID
}

// NewDiscordUpdater creates a new Discord presence updater
func NewDiscordUpdater(cfg *config.PlatformConfig) *DiscordUpdater {
	if cfg == nil {
		return &DiscordUpdater{enabled: false}
	}

	clientID := ""
	if cfg.Extra != nil {
		clientID = cfg.Extra["client_id"]
	}

	return &DiscordUpdater{
		enabled:  cfg.Enabled,
		clientID: clientID,
	}
}

// Name returns the platform name
func (d *DiscordUpdater) Name() string {
	return "discord"
}

// IsEnabled returns whether Discord is enabled
func (d *DiscordUpdater) IsEnabled() bool {
	return d.enabled && d.clientID != ""
}

// UpdatePresence updates the user's Discord Rich Presence via the local IPC socket
func (d *DiscordUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
	if !d.IsEnabled() {
		return fmt.Errorf("discord is not enabled or client_id is missing")
	}

	payload := d.buildPresencePayload(update)
	return d.sendCommand(payload)
}

// ClearPresence clears the Discord Rich Presence
func (d *DiscordUpdater) ClearPresence(ctx context.Context) error {
	if !d.IsEnabled() {
		return fmt.Errorf("discord is not enabled or client_id is missing")
	}

	payload := map[string]interface{}{
		"cmd": "SET_ACTIVITY",
		"args": map[string]interface{}{
			"pid":      os.Getpid(),
			"activity": nil,
		},
		"nonce": newNonce(),
	}
	return d.sendCommand(payload)
}

// sendCommand connects to the Discord IPC socket, performs the required
// handshake, sends the given command, and reads back Discord's response
// to detect errors. Each call uses a fresh connection since Discord Rich
// Presence persists after the socket closes (until cleared or replaced).
// KNOWN GAP if this is ever un-paused: getDiscordSocketPath() below returns
// a plausible-looking Windows named-pipe path (`\\.\pipe\discord-ipc-0`),
// but this dial call is written for Unix-domain-socket semantics
// (net.DialUnix, Net: "unix"), which is not how Windows named pipes work.
// It compiles fine on Windows (confirmed via cross-platform release build
// testing - not a build blocker), but would almost certainly fail to
// actually connect at runtime. Fixing this for real means using a Windows
// named-pipe library (e.g. github.com/Microsoft/go-winio) behind a
// build-tagged file, not just tweaking this call. See HANDOFF.md §4.4/§5.6.
func (d *DiscordUpdater) sendCommand(payload map[string]interface{}) error {
	socketPath, err := d.getDiscordSocketPath()
	if err != nil {
		return fmt.Errorf("failed to find Discord socket: %w", err)
	}

	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return fmt.Errorf("failed to connect to Discord IPC socket at %s: %w", socketPath, err)
	}
	defer conn.Close()

	// Step 1: Handshake. This MUST be the first message on a new
	// connection, or Discord will ignore/reject subsequent commands.
	handshake := map[string]interface{}{
		"v":         1,
		"client_id": d.clientID,
	}
	if err := writeIPCFrame(conn, opHandshake, handshake); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	_, readyBody, err := readIPCFrame(conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}
	if err := checkDiscordError(readyBody); err != nil {
		return fmt.Errorf("handshake rejected: %w", err)
	}

	// Step 2: Send the actual command as a FRAME message.
	if err := writeIPCFrame(conn, opFrame, payload); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	_, respBody, err := readIPCFrame(conn)
	if err != nil {
		return fmt.Errorf("failed to read command response: %w", err)
	}
	if err := checkDiscordError(respBody); err != nil {
		return fmt.Errorf("discord rejected command: %w", err)
	}

	return nil
}

// checkDiscordError inspects a Discord IPC response body for an error
// event (evt: "ERROR") and returns it as a Go error if present.
func checkDiscordError(body []byte) error {
	var resp struct {
		Evt  string `json:"evt"`
		Data struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		// Not all responses are easily typed; if we can't parse it,
		// don't block on it - better to be lenient than to break
		// otherwise-successful updates due to a parsing edge case.
		return nil
	}
	if resp.Evt == "ERROR" {
		return fmt.Errorf("discord error %d: %s", resp.Data.Code, resp.Data.Message)
	}
	return nil
}

// buildPresencePayload constructs the RPC presence payload.
// Empty string fields are omitted since Discord's Rich Presence schema
// rejects empty (but present) string values for activity fields.
func (d *DiscordUpdater) buildPresencePayload(update PresenceUpdate) map[string]interface{} {
	details := update.Status
	if update.Emoji != "" {
		if details != "" {
			details = fmt.Sprintf("%s %s", update.Emoji, details)
		} else {
			details = update.Emoji
		}
	}

	activity := map[string]interface{}{
		"state": d.mapStateToDiscordState(update.State),
	}
	if details != "" {
		activity["details"] = details
	}

	return map[string]interface{}{
		"cmd": "SET_ACTIVITY",
		"args": map[string]interface{}{
			"pid":      os.Getpid(),
			"activity": activity,
		},
		"nonce": newNonce(),
	}
}

// newNonce generates a unique nonce string required by Discord's RPC
// protocol for FRAME (opcode 1) commands, used to match requests to
// their corresponding responses.
func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely, but fall back to a fixed-but-unique-enough
		// value rather than failing the whole command.
		return fmt.Sprintf("%x", os.Getpid())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// mapStateToDiscordState converts our PresenceState to Discord's state string
func (d *DiscordUpdater) mapStateToDiscordState(state PresenceState) string {
	switch state {
	case StateActive:
		return "Active"
	case StateAway:
		return "Away"
	case StateDND:
		return "Do Not Disturb"
	case StateBusy:
		return "Busy"
	case StateBRB:
		return "Be Right Back"
	case StateOffline:
		return "Offline"
	default:
		return "Active"
	}
}

// IPC opcodes per Discord's RPC protocol
const (
	opHandshake = 0
	opFrame     = 1
)

// writeIPCFrame writes a single Discord IPC message frame:
// [opcode (4 bytes, little-endian)][length (4 bytes, little-endian)][JSON data]
//
// NOTE: opcodes and length are 4-byte fields, not 1-byte as a naive reading
// of the protocol might suggest - this was a latent bug in the original
// implementation (opcode was written as a single byte).
func writeIPCFrame(conn net.Conn, opcode uint32, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	message := make([]byte, 0, 8+len(jsonData))
	message = append(message, le32(opcode)...)
	message = append(message, le32(uint32(len(jsonData)))...)
	message = append(message, jsonData...)

	_, err = conn.Write(message)
	return err
}

// readIPCFrame reads a single Discord IPC message frame and returns the
// opcode and raw JSON body.
func readIPCFrame(conn net.Conn) (uint32, []byte, error) {
	header := make([]byte, 8)
	if _, err := readFull(conn, header); err != nil {
		return 0, nil, fmt.Errorf("failed to read frame header: %w", err)
	}

	opcode := decodeLE32(header[0:4])
	length := decodeLE32(header[4:8])

	body := make([]byte, length)
	if length > 0 {
		if _, err := readFull(conn, body); err != nil {
			return 0, nil, fmt.Errorf("failed to read frame body: %w", err)
		}
	}

	return opcode, body, nil
}

// readFull reads exactly len(buf) bytes from conn.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func le32(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func decodeLE32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// getDiscordSocketPath returns the path to the Discord RPC socket
// Discord creates IPC sockets at different locations depending on OS
func (d *DiscordUpdater) getDiscordSocketPath() (string, error) {
	if runtime.GOOS == "windows" {
		// On Windows, the socket is a named pipe (no indexed search needed
		// here; if this becomes an issue we can extend it similarly).
		return "\\\\.\\pipe\\discord-ipc-0", nil
	}

	runtimeDir, err := discordRuntimeDir()
	if err != nil {
		return "", err
	}

	// Discord (and other IPC clients on the same machine, e.g. multiple
	// Discord/Canary/PTB installs) each grab the next available slot, so
	// the socket isn't always discord-ipc-0. Try a handful of indices.
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(runtimeDir, fmt.Sprintf("discord-ipc-%d", i))
		if info, err := os.Stat(candidate); err == nil && (info.Mode()&os.ModeSocket) != 0 {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no discord-ipc-N socket found in %s. Is Discord running?", runtimeDir)
}

// discordRuntimeDir returns the directory where Discord places its IPC
// sockets. On both macOS and Linux, Electron-based apps like Discord use
// a per-user temp/runtime directory - NOT ~/Library/Application Support
// on macOS, despite that being a common assumption.
func discordRuntimeDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Discord uses $TMPDIR (e.g. /var/folders/xx/.../T/)
		if tmpDir := os.Getenv("TMPDIR"); tmpDir != "" {
			return tmpDir, nil
		}
		return "/tmp", nil
	case "linux":
		// Linux: Discord uses $XDG_RUNTIME_DIR, falling back to /tmp
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			return runtimeDir, nil
		}
		return "/tmp", nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
