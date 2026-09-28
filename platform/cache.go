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
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/SinisterNerd/omnistatus/config"
)

// defaultCacheTTL is used when caching is enabled but no ttl is configured.
const defaultCacheTTL = 10 * time.Second

// CacheEntry stores a single platform's last-fetched presence info, along
// with when it was fetched (to determine staleness against the TTL).
type CacheEntry struct {
	Info      PresenceInfo `json:"info"`
	FetchedAt time.Time    `json:"fetched_at"`
}

// CacheFile is the on-disk shape of the status cache: one entry per
// platform name (e.g., "teams", "slack", "github"). Shared on disk between
// the CLI (`ost status`) and the menu bar app so they don't double-hit the
// live APIs when both are polling around the same time.
type CacheFile struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// getCacheFilePath returns the path to the status cache file, colocated
// with the currently active config file. This means a portable config
// (via --config) gets its own independent cache rather than sharing one
// tied to the default ~/.config/omnistatus location.
func getCacheFilePath() (string, error) {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), ".status_cache.json"), nil
}

// LoadCache reads the cache file from disk. Any error (missing file,
// corrupt JSON, etc.) is treated as an empty cache rather than a fatal
// error - caching is a pure optimization, never a hard dependency.
func LoadCache() CacheFile {
	empty := CacheFile{Entries: make(map[string]CacheEntry)}

	path, err := getCacheFilePath()
	if err != nil {
		return empty
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return empty
	}

	var cache CacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return empty
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}
	return cache
}

// SaveCache writes the cache file to disk atomically (write to a temp
// file, then rename over the target) so that concurrent readers - e.g.,
// multiple tmux panes or the menu bar app polling at once - never observe
// a partially written/corrupt file.
func SaveCache(cache CacheFile) error {
	path, err := getCacheFilePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ResolveCacheSettings reads the [cache] config section and returns
// whether caching is enabled and what TTL to use. If enabled but no TTL is
// configured (or it fails to parse), defaultCacheTTL is used.
func ResolveCacheSettings(cfg *config.Config) (enabled bool, ttl time.Duration) {
	if cfg.Cache == nil || !cfg.Cache.Enabled {
		return false, 0
	}

	ttl = defaultCacheTTL
	if cfg.Cache.TTL != "" {
		if parsed, err := time.ParseDuration(cfg.Cache.TTL); err == nil && parsed > 0 {
			ttl = parsed
		}
	}
	return true, ttl
}

// GetCached returns presence info for a single platform, using a cached
// value if caching is enabled and the cached entry is still within its
// TTL. Otherwise it fetches fresh and updates the cache (marking
// cacheDirty so the caller knows to persist it).
func GetCached(ctx context.Context, name string, reader PresenceReader, cacheEnabled bool, ttl time.Duration, cache *CacheFile, cacheDirty *bool) (PresenceInfo, bool, error) {
	if cacheEnabled {
		if entry, ok := cache.Entries[name]; ok {
			if time.Since(entry.FetchedAt) < ttl {
				return entry.Info, true, nil
			}
		}
	}

	info, err := reader.GetPresence(ctx)
	if err != nil {
		return PresenceInfo{}, false, err
	}

	if cacheEnabled {
		cache.Entries[name] = CacheEntry{Info: info, FetchedAt: time.Now()}
		*cacheDirty = true
	}

	return info, false, nil
}
