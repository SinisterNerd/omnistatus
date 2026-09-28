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
	"context"
	"fmt"
	"time"

	"github.com/SinisterNerd/omnistatus/config"
	"github.com/SinisterNerd/omnistatus/platform"
)

// getTeamsUpdater creates a TeamsUpdater from config, automatically running
// the interactive device-code sign-in flow if credentials are missing
// (e.g., first time Teams is enabled, or if the refresh token was revoked).
//
// The device-code flow works over SSH: it just prints a URL and short code
// that can be completed from any browser on any device - no local server
// or browser on the omniStatus machine is required.
func getTeamsUpdater(cfg *config.PlatformConfig) (*platform.TeamsUpdater, error) {
	updater := platform.NewTeamsUpdater(cfg)

	if updater.NeedsInteractiveAuth() {
		// Device code flow waits on user input, so give it a generous timeout.
		authCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := updater.AuthenticateInteractive(authCtx); err != nil {
			return nil, fmt.Errorf("teams authentication failed: %w", err)
		}
	}

	return updater, nil
}
