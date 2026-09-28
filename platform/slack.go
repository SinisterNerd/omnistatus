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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/SinisterNerd/omnistatus/config"
)

// SlackUpdater implements PresenceUpdater for Slack
type SlackUpdater struct {
	enabled bool
	token   string
}

// NewSlackUpdater creates a new Slack presence updater
func NewSlackUpdater(cfg *config.PlatformConfig) *SlackUpdater {
	if cfg == nil {
		return &SlackUpdater{enabled: false}
	}
	return &SlackUpdater{
		enabled: cfg.Enabled,
		token:   cfg.Token,
	}
}

// Name returns the platform name
func (s *SlackUpdater) Name() string {
	return "slack"
}

// IsEnabled returns whether Slack is enabled
func (s *SlackUpdater) IsEnabled() bool {
	return s.enabled && s.token != ""
}

// UpdatePresence updates the user's Slack status and presence
func (s *SlackUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
	if !s.IsEnabled() {
		return fmt.Errorf("slack is not enabled or token is missing")
	}

	// Map our state to Slack's presence values
	slackPresence := s.mapStateToSlackPresence(update.State)

	// First update the user's profile status/emoji
	if err := s.updateProfileStatus(ctx, update.Status, update.Emoji); err != nil {
		return fmt.Errorf("failed to update profile status: %w", err)
	}

	// Then set the presence status
	if err := s.setPresence(ctx, slackPresence); err != nil {
		return fmt.Errorf("failed to set presence: %w", err)
	}

	return nil
}

// ClearPresence clears the Slack status
func (s *SlackUpdater) ClearPresence(ctx context.Context) error {
	if !s.IsEnabled() {
		return fmt.Errorf("slack is not enabled or token is missing")
	}

	// Clear profile status
	if err := s.updateProfileStatus(ctx, "", ""); err != nil {
		return fmt.Errorf("failed to clear profile status: %w", err)
	}

	// Set presence to auto (let Slack determine based on activity)
	if err := s.setPresence(ctx, "auto"); err != nil {
		return fmt.Errorf("failed to reset presence: %w", err)
	}

	return nil
}

// slackAPIErrorCodeInvalidEmoji is the error code Slack's users.profile.set
// returns when status_emoji isn't a valid emoji for this workspace (a typo,
// or a :code: that's valid elsewhere but not a standard/custom emoji here).
const slackAPIErrorCodeInvalidEmoji = "profile_status_set_failed_not_valid_emoji"

// updateProfileStatus updates the user's profile status message and emoji.
// If Slack rejects the emoji specifically (invalid/unrecognized code), this
// retries once with the emoji dropped so the status *text* still gets set
// rather than failing the whole update over a cosmetic detail - the same
// "degrade gracefully instead of hard-failing" approach used elsewhere in
// this project (the tmux "?" fallback, GitHub's default-busy-emoji
// fallback). A genuinely bad token, network error, etc. still fails
// normally - only this one specific, recoverable error code gets a retry.
func (s *SlackUpdater) updateProfileStatus(ctx context.Context, status, emoji string) error {
	// Slack Web API endpoint
	const slackAPIEndpoint = "https://slack.com/api/users.profile.set"

	buildPayload := func(emoji string) map[string]interface{} {
		return map[string]interface{}{
			"profile": map[string]string{
				"status_text":  status,
				"status_emoji": emoji,
			},
		}
	}

	err := s.doRequest(ctx, slackAPIEndpoint, buildPayload(emoji))
	if err == nil {
		return nil
	}

	var apiErr *slackAPIError
	if emoji == "" || !errors.As(err, &apiErr) || apiErr.code != slackAPIErrorCodeInvalidEmoji {
		return err
	}

	if retryErr := s.doRequest(ctx, slackAPIEndpoint, buildPayload("")); retryErr != nil {
		// The retry failed too - return the *original* error, since it's
		// the more informative one (names the actual problem: the emoji).
		return err
	}

	fmt.Printf("slack: warning: emoji %q was rejected as invalid, status text set without it\n", emoji)
	return nil
}

// setPresence sets the user's presence status (away, active, auto)
func (s *SlackUpdater) setPresence(ctx context.Context, presence string) error {
	// Slack Web API endpoint
	const slackAPIEndpoint = "https://slack.com/api/users.setPresence"

	payload := map[string]interface{}{
		"presence": presence,
	}

	return s.doRequest(ctx, slackAPIEndpoint, payload)
}

// slackAPIError wraps a Slack Web API error response, exposing the raw
// error code (e.g. "profile_status_set_failed_not_valid_emoji") so callers
// can react to specific failures instead of only string-matching the
// formatted error message. Error() intentionally matches the original
// plain "slack API error: %s" format so this is a transparent change for
// any caller that doesn't specifically check the code via errors.As.
type slackAPIError struct {
	code string
}

func (e *slackAPIError) Error() string {
	return fmt.Sprintf("slack API error: %s", e.code)
}

// doRequest performs a POST request to the Slack API
func (s *SlackUpdater) doRequest(ctx context.Context, endpoint string, payload map[string]interface{}) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	// Perform request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to check for Slack API errors
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	ok, exists := response["ok"].(bool)
	if !exists || !ok {
		errMsg, _ := response["error"].(string)
		return &slackAPIError{code: errMsg}
	}

	return nil
}

// GetPresence retrieves the user's current Slack presence and status.
func (s *SlackUpdater) GetPresence(ctx context.Context) (PresenceInfo, error) {
	if !s.IsEnabled() {
		return PresenceInfo{}, fmt.Errorf("slack is not enabled or token is missing")
	}

	// users.getPresence without a "user" param returns the authenticated
	// user's own presence (active/away).
	presenceResp, err := s.doGetRequest(ctx, "https://slack.com/api/users.getPresence")
	if err != nil {
		return PresenceInfo{}, fmt.Errorf("failed to get presence: %w", err)
	}

	var presenceResult struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Presence string `json:"presence"`
	}
	if err := json.Unmarshal(presenceResp, &presenceResult); err != nil {
		return PresenceInfo{}, fmt.Errorf("failed to parse presence response: %w", err)
	}
	if !presenceResult.OK {
		return PresenceInfo{}, fmt.Errorf("slack API error: %s", presenceResult.Error)
	}

	// users.profile.get without a "user" param returns the authenticated
	// user's own profile, which includes the custom status text/emoji.
	// This requires the "users.profile:read" scope. If that scope isn't
	// granted, we don't fail the whole call - presence (active/away) is
	// the important part, and status text is just a nice-to-have.
	activity := ""
	if profileResp, err := s.doGetRequest(ctx, "https://slack.com/api/users.profile.get"); err == nil {
		var profileResult struct {
			OK      bool   `json:"ok"`
			Error   string `json:"error"`
			Profile struct {
				StatusText  string `json:"status_text"`
				StatusEmoji string `json:"status_emoji"`
			} `json:"profile"`
		}
		if err := json.Unmarshal(profileResp, &profileResult); err == nil && profileResult.OK {
			activity = profileResult.Profile.StatusText
			if profileResult.Profile.StatusEmoji != "" {
				activity = fmt.Sprintf("%s %s", profileResult.Profile.StatusEmoji, activity)
			}
		}
	}

	return PresenceInfo{
		Platform:     "slack",
		Availability: presenceResult.Presence,
		Activity:     activity,
	}, nil
}

// doGetRequest performs a simple authenticated GET request to a Slack API endpoint
func (s *SlackUpdater) doGetRequest(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// mapStateToSlackPresence converts our PresenceState to Slack's presence values
// Slack's users.setPresence API only accepts two enum values: "auto"
// (online/active) and "away". There is no literal "active" value.
func (s *SlackUpdater) mapStateToSlackPresence(state PresenceState) string {
	switch state {
	case StateActive:
		return "auto"
	case StateAway:
		return "away"
	case StateDND:
		return "away" // Slack doesn't have a distinct DND state, we use away
	case StateBusy:
		return "away" // Slack has no "busy" state, closest is away
	case StateBRB:
		return "away" // Slack has no "be right back" state, closest is away
	case StateOffline:
		return "away" // Slack API can't force fully offline, closest is away
	default:
		return "auto"
	}
}
