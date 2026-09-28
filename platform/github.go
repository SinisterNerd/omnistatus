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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rspence/omnistatus/config"
)

// GitHubUpdater implements PresenceUpdater for GitHub's profile status.
//
// GitHub has no concept of "availability state" (no online/away/busy
// indicator) - the ONLY thing it exposes is the profile status: an emoji
// + message combo, optionally with an expiration time and a
// "limitedAvailability" flag (the checkbox GitHub's own UI labels
// "Busy"). Because of this, GitHub is the one platform where omitting
// --status/--emoji means nothing will visibly change - the --state flag
// only affects the limitedAvailability boolean (see mapStateToLimitedAvailability).
//
// Uses GitHub's GraphQL API (changeUserStatus mutation / viewer.status
// query) with a Personal Access Token.
type GitHubUpdater struct {
	enabled bool
	token   string
}

const githubGraphQLEndpoint = "https://api.github.com/graphql"

// NewGitHubUpdater creates a new GitHub presence updater
func NewGitHubUpdater(cfg *config.PlatformConfig) *GitHubUpdater {
	if cfg == nil {
		return &GitHubUpdater{enabled: false}
	}
	return &GitHubUpdater{
		enabled: cfg.Enabled,
		token:   cfg.Token,
	}
}

// Name returns the platform name
func (g *GitHubUpdater) Name() string {
	return "github"
}

// IsEnabled returns whether GitHub is enabled
func (g *GitHubUpdater) IsEnabled() bool {
	return g.enabled && g.token != ""
}

// UpdatePresence sets the user's GitHub profile status (emoji + message),
// optionally with an expiration time and limited-availability flag.
func (g *GitHubUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
	if !g.IsEnabled() {
		return fmt.Errorf("github is not enabled or token is missing")
	}

	emoji := update.Emoji
	limitedAvailability := g.mapStateToLimitedAvailability(update.State)
	// GitHub's changeUserStatus mutation silently no-ops - returns
	// status: null and persists nothing, not even limitedAvailability -
	// when both emoji and message are empty (confirmed via live GraphQL
	// testing). So a state-only "mark busy" with no --status/--emoji
	// would otherwise appear to succeed but have zero visible effect.
	// Supply a default emoji in that case so the busy flag actually
	// sticks; any user-supplied emoji or message is left untouched.
	if limitedAvailability && emoji == "" && update.Status == "" {
		emoji = ":no_entry:"
	}

	input := map[string]interface{}{
		"emoji":               emoji,
		"message":             update.Status,
		"limitedAvailability": limitedAvailability,
	}
	if update.HasDuration() {
		input["expiresAt"] = update.ExpiresAt().Format(time.RFC3339)
	}

	return g.changeUserStatus(ctx, input)
}

// ClearPresence clears the user's GitHub profile status entirely.
func (g *GitHubUpdater) ClearPresence(ctx context.Context) error {
	if !g.IsEnabled() {
		return fmt.Errorf("github is not enabled or token is missing")
	}

	input := map[string]interface{}{
		"emoji":               "",
		"message":             "",
		"limitedAvailability": false,
	}

	return g.changeUserStatus(ctx, input)
}

// GetPresence retrieves the user's current GitHub profile status.
func (g *GitHubUpdater) GetPresence(ctx context.Context) (PresenceInfo, error) {
	if !g.IsEnabled() {
		return PresenceInfo{}, fmt.Errorf("github is not enabled or token is missing")
	}

	// NOTE: the read-side field is named "indicatesLimitedAvailability",
	// which differs from the mutation input field "limitedAvailability"
	// (confirmed via GraphQL schema introspection).
	query := `query {
		viewer {
			status {
				message
				emoji
				expiresAt
				indicatesLimitedAvailability
			}
		}
	}`

	respBody, err := g.doGraphQLRequest(ctx, query, nil)
	if err != nil {
		return PresenceInfo{}, err
	}

	var result struct {
		Data struct {
			Viewer struct {
				Status *struct {
					Message                      string `json:"message"`
					Emoji                        string `json:"emoji"`
					ExpiresAt                    string `json:"expiresAt"`
					IndicatesLimitedAvailability bool   `json:"indicatesLimitedAvailability"`
				} `json:"status"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return PresenceInfo{}, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Errors) > 0 {
		return PresenceInfo{}, fmt.Errorf("github API error: %s", result.Errors[0].Message)
	}

	info := PresenceInfo{Platform: "github"}

	if result.Data.Viewer.Status == nil {
		info.Availability = "none"
		return info, nil
	}

	status := result.Data.Viewer.Status
	if status.IndicatesLimitedAvailability {
		info.Availability = "busy"
	} else {
		info.Availability = "available"
	}

	activity := status.Message
	if status.Emoji != "" {
		activity = fmt.Sprintf("%s %s", status.Emoji, activity)
	}
	info.Activity = activity

	if status.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, status.ExpiresAt); err == nil {
			info.ExpiresAt = &t
		}
	}

	return info, nil
}

// mapStateToLimitedAvailability converts our PresenceState to GitHub's
// single "limitedAvailability" boolean (the "Busy" checkbox in GitHub's
// UI). GitHub has no finer-grained state than this, so any state other
// than "active" is treated as limited availability.
func (g *GitHubUpdater) mapStateToLimitedAvailability(state PresenceState) bool {
	return state != StateActive
}

// changeUserStatus performs the changeUserStatus GraphQL mutation.
func (g *GitHubUpdater) changeUserStatus(ctx context.Context, input map[string]interface{}) error {
	// NOTE: the input field is "limitedAvailability" (ChangeUserStatusInput),
	// but the response/output field is "indicatesLimitedAvailability"
	// (UserStatus) - these are genuinely different names in GitHub's schema.
	mutation := `mutation($input: ChangeUserStatusInput!) {
		changeUserStatus(input: $input) {
			status {
				message
				emoji
				expiresAt
				indicatesLimitedAvailability
			}
		}
	}`

	variables := map[string]interface{}{"input": input}

	respBody, err := g.doGraphQLRequest(ctx, mutation, variables)
	if err != nil {
		return err
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("github API error: %s", result.Errors[0].Message)
	}

	return nil
}

// doGraphQLRequest performs a request against GitHub's GraphQL API.
func (g *GitHubUpdater) doGraphQLRequest(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubGraphQLEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
