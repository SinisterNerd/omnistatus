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

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PlatformConfig represents configuration for a specific platform
type PlatformConfig struct {
	Enabled bool               `yaml:"enabled"`
	Token   string             `yaml:"token"`
	Extra   map[string]string  `yaml:"extra"`          // For platform-specific config
	Tmux    *TmuxDisplayConfig `yaml:"tmux,omitempty"` // Per-platform tmux status-line display (see `ost status --format tmux`)
}

// TmuxDisplayConfig controls how a single platform is rendered by
// `ost status --format tmux` and, on Windows, the tray app's floating
// status window (`cmd/traywindows`). The glyph/character shown for that
// platform, the font it's drawn in (native rendering only - see Font
// below), and the color used for each of the three availability buckets
// (green/yellow/red). Any field left blank falls back to a sane default,
// so this whole block - and every field in it - is optional.
//
// Colors accept anything tmux's `#[fg=...]` understands: a color name
// ("green", "brightred"), a tmux 256-color index ("colour208"), or (on
// tmux 2.9+ with a truecolor terminal) a hex value ("#ff8800"). The
// Windows tray app's native renderer understands hex and a handful of
// common color names, but not tmux's 256-color palette indices - an
// unrecognized value there just falls back to the plain default color for
// that bucket.
type TmuxDisplayConfig struct {
	Icon        string `yaml:"icon,omitempty"`         // e.g. "" (Nerd Font glyph), "S", "●"
	ColorGreen  string `yaml:"color_green,omitempty"`  // used when availability maps to "available"
	ColorYellow string `yaml:"color_yellow,omitempty"` // used when availability maps to "away/transitional"
	ColorRed    string `yaml:"color_red,omitempty"`    // used when availability maps to "busy/dnd/offline"

	// Font is the font family name to draw Icon with. Ignored by tmux
	// (which always uses the terminal's own font) - only meaningful for
	// the Windows tray app's floating status window, which does its own
	// native text rendering and can point at any font actually installed
	// on that machine, e.g. a Nerd Font, so the real configured glyph can
	// render there too (not just a plain-letter fallback). Must be
	// installed on the Windows machine to have any effect; if empty, or
	// the named font isn't found, falls back to the system UI font, which
	// almost never has the Nerd Font glyph codepoints.
	Font string `yaml:"font,omitempty"`
}

// Icon returns the configured tmux icon for this platform, or fallback if
// none is set (nil-safe: a platform with no config block at all, or no
// tmux sub-block, just uses the fallback).
func (p *PlatformConfig) Icon(fallback string) string {
	if p != nil && p.Tmux != nil && p.Tmux.Icon != "" {
		return p.Tmux.Icon
	}
	return fallback
}

// Font returns the configured font family name for this platform's icon
// (Windows tray app only - see TmuxDisplayConfig.Font), or fallback if
// none is set.
func (p *PlatformConfig) Font(fallback string) string {
	if p != nil && p.Tmux != nil && p.Tmux.Font != "" {
		return p.Tmux.Font
	}
	return fallback
}

// ColorFor returns the configured tmux color for the given bucket
// ("green", "yellow", or "red"), or that bucket's plain color name as a
// default if none is configured.
func (p *PlatformConfig) ColorFor(bucket string) string {
	if p != nil && p.Tmux != nil {
		switch bucket {
		case "green":
			if p.Tmux.ColorGreen != "" {
				return p.Tmux.ColorGreen
			}
		case "yellow":
			if p.Tmux.ColorYellow != "" {
				return p.Tmux.ColorYellow
			}
		case "red":
			if p.Tmux.ColorRed != "" {
				return p.Tmux.ColorRed
			}
		}
	}
	return bucket
}

// CacheConfig controls local caching of `ost status` results, used to
// avoid redundant API calls when multiple callers (e.g., several tmux
// panes/sessions each polling their status bar) query status at the same
// time. This is purely a local read-cache - it never affects `ost set`
// or `ost clear`, which always make live requests.
type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"` // duration string, e.g. "10s", "30s" - parsed via time.ParseDuration
}

// Config represents the entire omniStatus configuration
type Config struct {
	Slack   *PlatformConfig `yaml:"slack"`
	Teams   *PlatformConfig `yaml:"teams"`
	Discord *PlatformConfig `yaml:"discord"`
	GitHub  *PlatformConfig `yaml:"github"`
	Cache   *CacheConfig    `yaml:"cache"`
}

// configPathOverride, when set via SetConfigPath, takes precedence over
// the default ~/.config/omnistatus/config.yaml location. This allows
// running omniStatus with a portable config (e.g., on a USB drive or a
// shared/multi-user server) via the --config flag.
var configPathOverride string

// SetConfigPath overrides the config file path for the remainder of the
// process. Pass an empty string to revert to the default location.
func SetConfigPath(path string) {
	configPathOverride = path
}

// LoadConfig loads the configuration from the YAML file. By default this
// is ~/.config/omnistatus/config.yaml, unless overridden via SetConfigPath.
func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine config path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found at %s. Please create it first", configPath)
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return &cfg, nil
}

// SaveConfig writes the configuration to the YAML file
func SaveConfig(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the config file, honoring any
// override set via SetConfigPath (e.g., from the --config CLI flag).
func getConfigPath() (string, error) {
	if configPathOverride != "" {
		return configPathOverride, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "omnistatus", "config.yaml"), nil
}

// GetConfigPath returns the resolved path to the config file currently in
// use (honoring any --config override). Exported so other packages (e.g.
// the status cache) can derive paths relative to the config's location.
func GetConfigPath() (string, error) {
	return getConfigPath()
}
