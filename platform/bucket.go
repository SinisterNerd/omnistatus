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

// Bucket normalizes a platform's raw, platform-specific availability string
// into one of three buckets - "green" (available), "yellow" (away/
// transitional), or "red" (busy/dnd/offline). Used by both `ost status
// --format tmux` and the menu bar app to decide what color/glyph to show.
// Falls back to "yellow" for any value not explicitly recognized (e.g. a
// new Graph API presence value Microsoft adds later), since that's the
// least alarming default.
func Bucket(platformName, availability string) string {
	switch platformName {
	case "slack":
		switch availability {
		case "active":
			return "green"
		case "away":
			return "yellow"
		}
	case "teams":
		switch availability {
		case "Available", "AvailableIdle":
			return "green"
		case "Away", "AwayIdle", "BeRightBack":
			return "yellow"
		case "Busy", "BusyIdle", "DoNotDisturb", "DoNotDisturbIdle", "Offline", "PresenceUnknown":
			return "red"
		}
	case "github":
		switch availability {
		case "available", "none":
			return "green"
		case "busy":
			return "red"
		}
	}
	return "yellow"
}
