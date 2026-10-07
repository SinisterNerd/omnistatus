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
	"fmt"
	"sort"
	"strings"

	"github.com/SinisterNerd/omnistatus/config"
	"github.com/SinisterNerd/omnistatus/platform"
)

// buildManager registers every platform instance for `ost set`/`ost clear`.
// If only is non-empty, just those instance names are targeted (names as
// shown by `ost status`, e.g. "slack", "github-personal"); each must exist
// and be enabled, otherwise it's an error listing the valid choices rather
// than silently doing nothing. Teams' interactive sign-in is only attempted
// when Teams is actually targeted.
func buildManager(cfg *config.Config, only []string) (*platform.Manager, error) {
	wanted := make(map[string]bool, len(only))
	for _, name := range only {
		wanted[strings.TrimSpace(name)] = true
	}
	targeted := func(name string) bool { return len(wanted) == 0 || wanted[name] }

	instanceUpdaters, err := platform.NewInstanceUpdaters(cfg)
	if err != nil {
		return nil, err
	}

	all := []platform.PresenceUpdater{platform.NewSlackUpdater(cfg.Slack)}
	if targeted("teams") {
		teamsUpdater, err := getTeamsUpdater(cfg.Teams)
		if err != nil {
			return nil, err
		}
		all = append(all, teamsUpdater)
	} else {
		all = append(all, platform.NewTeamsUpdater(cfg.Teams))
	}
	all = append(all, platform.NewDiscordUpdater(cfg.Discord), platform.NewGitHubUpdater(cfg.GitHub))
	all = append(all, instanceUpdaters...)

	manager := platform.NewManager()
	var enabledNames []string
	for _, u := range all {
		if u.IsEnabled() {
			enabledNames = append(enabledNames, u.Name())
		}
		if targeted(u.Name()) {
			manager.Register(u)
		}
	}

	sort.Strings(enabledNames)
	for name := range wanted {
		found := false
		for _, enabled := range enabledNames {
			if enabled == name {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("platform %q is not configured/enabled (enabled: %s)", name, strings.Join(enabledNames, ", "))
		}
	}
	return manager, nil
}
