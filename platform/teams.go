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
	"net/url"
	"strings"
	"time"

	"github.com/rspence/omnistatus/config"
)

// TeamsUpdater implements PresenceUpdater for Microsoft Teams.
//
// It uses delegated OAuth 2.0 authentication via the Device Code Flow.
// This flow doesn't require a local redirect/callback server, so it works
// seamlessly over SSH: the user can complete sign-in on any device, not
// necessarily the machine running omniStatus.
//
// Microsoft Graph's presence-setting endpoints only work reliably with
// delegated auth on the /me endpoint (not app-only/client-credentials).
// Access tokens are short-lived (~1 hour); this updater automatically
// refreshes them using the stored refresh token and persists the new
// tokens back to the config file.
//
// NOTE: Microsoft Graph API does not currently support setting a custom
// status message (the text shown under your name in Teams). Only the
// availability state can be set via the API.
type TeamsUpdater struct {
	enabled      bool
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	tenantID     string
}

const teamsGraphScope = "https://graph.microsoft.com/Presence.ReadWrite https://graph.microsoft.com/User.Read offline_access"

// NewTeamsUpdater creates a new Microsoft Teams presence updater
func NewTeamsUpdater(cfg *config.PlatformConfig) *TeamsUpdater {
	if cfg == nil {
		return &TeamsUpdater{enabled: false}
	}

	get := func(key string) string {
		if cfg.Extra == nil {
			return ""
		}
		return cfg.Extra[key]
	}

	return &TeamsUpdater{
		enabled:      cfg.Enabled,
		accessToken:  cfg.Token,
		refreshToken: get("refresh_token"),
		clientID:     get("client_id"),
		clientSecret: get("client_secret"),
		tenantID:     get("tenant_id"),
	}
}

// Name returns the platform name
func (t *TeamsUpdater) Name() string {
	return "teams"
}

// IsEnabled returns whether Teams is enabled and has usable credentials
func (t *TeamsUpdater) IsEnabled() bool {
	return t.enabled && t.accessToken != "" && t.refreshToken != "" &&
		t.clientID != "" && t.tenantID != ""
}

// NeedsInteractiveAuth returns true if Teams is turned on and configured
// with the minimum required app registration info (client_id/tenant_id),
// but is missing the tokens needed to actually make API calls. In this
// case, the caller should run AuthenticateInteractive before proceeding.
func (t *TeamsUpdater) NeedsInteractiveAuth() bool {
	return t.enabled && t.clientID != "" && t.tenantID != "" &&
		(t.accessToken == "" || t.refreshToken == "")
}

// UpdatePresence updates the user's Teams availability.
//
// Only availability is set. Microsoft Graph does not support setting a
// custom status message via the API, so the Status/Emoji fields of the
// update are intentionally ignored for Teams.
func (t *TeamsUpdater) UpdatePresence(ctx context.Context, update PresenceUpdate) error {
	if !t.IsEnabled() {
		return fmt.Errorf("teams is not enabled or missing token/refresh_token/client_id/tenant_id")
	}

	availability, activity := t.mapStateToTeamsPresence(update.State)
	return t.setPreferredPresence(ctx, availability, activity, update.Duration)
}

// ClearPresence resets the user's Teams availability to the default
// (removes any preferred presence override).
func (t *TeamsUpdater) ClearPresence(ctx context.Context) error {
	if !t.IsEnabled() {
		return fmt.Errorf("teams is not enabled or missing token/refresh_token/client_id/tenant_id")
	}

	endpoint := "https://graph.microsoft.com/v1.0/me/presence/clearUserPreferredPresence"
	return t.doGraphRequest(ctx, endpoint, http.MethodPost, map[string]interface{}{})
}

// GetPresence retrieves the user's current Teams presence.
func (t *TeamsUpdater) GetPresence(ctx context.Context) (PresenceInfo, error) {
	if !t.IsEnabled() {
		return PresenceInfo{}, fmt.Errorf("teams is not enabled or missing token/refresh_token/client_id/tenant_id")
	}

	endpoint := "https://graph.microsoft.com/v1.0/me/presence"

	status, body, err := t.doGraphRequestOnceRaw(ctx, endpoint, http.MethodGet, nil)
	if err != nil {
		return PresenceInfo{}, err
	}

	if status == http.StatusUnauthorized {
		if refreshErr := t.refreshAccessToken(ctx); refreshErr != nil {
			return PresenceInfo{}, fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}
		status, body, err = t.doGraphRequestOnceRaw(ctx, endpoint, http.MethodGet, nil)
		if err != nil {
			return PresenceInfo{}, err
		}
	}

	if status < 200 || status >= 300 {
		return PresenceInfo{}, fmt.Errorf("graph API returned status %d: %s", status, string(body))
	}

	var result struct {
		Availability string `json:"availability"`
		Activity     string `json:"activity"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return PresenceInfo{}, fmt.Errorf("failed to parse presence response: %w", err)
	}

	return PresenceInfo{
		Platform:     "teams",
		Availability: result.Availability,
		Activity:     result.Activity,
	}, nil
}

// setPreferredPresence sets the user's preferred availability/activity
// using the delegated-auth-specific setUserPreferredPresence endpoint.
// If duration > 0, Microsoft Graph will automatically revert the
// preferred presence back to the user's actual/inferred presence after
// that period elapses (verified directly against the live API).
func (t *TeamsUpdater) setPreferredPresence(ctx context.Context, availability, activity string, duration time.Duration) error {
	endpoint := "https://graph.microsoft.com/v1.0/me/presence/setUserPreferredPresence"

	payload := map[string]interface{}{
		"availability": availability,
		"activity":     activity,
	}
	if duration > 0 {
		payload["expirationDuration"] = ISO8601Duration(duration)
	}

	return t.doGraphRequest(ctx, endpoint, http.MethodPost, payload)
}

// doGraphRequest performs a request to the Microsoft Graph API. If the
// request fails with a 401 (expired token), it automatically refreshes
// the access token, persists the new tokens to the config file, and
// retries the request once.
func (t *TeamsUpdater) doGraphRequest(ctx context.Context, endpoint string, method string, payload map[string]interface{}) error {
	status, body, err := t.doGraphRequestOnce(ctx, endpoint, method, payload)
	if err != nil {
		return err
	}

	if status == http.StatusUnauthorized {
		if refreshErr := t.refreshAccessToken(ctx); refreshErr != nil {
			return fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}

		status, body, err = t.doGraphRequestOnce(ctx, endpoint, method, payload)
		if err != nil {
			return err
		}
	}

	if status < 200 || status >= 300 {
		return fmt.Errorf("graph API returned status %d: %s", status, string(body))
	}

	return nil
}

// doGraphRequestOnce performs a single JSON HTTP request to Microsoft Graph.
func (t *TeamsUpdater) doGraphRequestOnce(ctx context.Context, endpoint string, method string, payload map[string]interface{}) (int, []byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}
	return t.doGraphRequestOnceRaw(ctx, endpoint, method, bytes.NewBuffer(jsonData))
}

// doGraphRequestOnceRaw performs a single HTTP request to Microsoft Graph
// with a raw body reader (or nil for no body), returning status + body.
func (t *TeamsUpdater) doGraphRequestOnceRaw(ctx context.Context, endpoint string, method string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// refreshAccessToken exchanges the stored refresh token for a new access
// token (and possibly a new refresh token, since Microsoft rotates them).
// The new tokens are persisted back to the config file.
func (t *TeamsUpdater) refreshAccessToken(ctx context.Context) error {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", t.tenantID)

	form := url.Values{}
	form.Set("client_id", t.clientID)
	form.Set("scope", teamsGraphScope)
	form.Set("refresh_token", t.refreshToken)
	form.Set("grant_type", "refresh_token")
	if t.clientSecret != "" {
		form.Set("client_secret", t.clientSecret)
	}

	tokenResp, err := t.postForToken(ctx, tokenURL, form)
	if err != nil {
		return err
	}

	t.accessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		t.refreshToken = tokenResp.RefreshToken
	}

	return t.persistTokens()
}

// AuthenticateInteractive performs the OAuth 2.0 Device Code Flow to
// obtain an initial access/refresh token pair. This is used the first
// time Teams is configured (or if the refresh token is ever revoked).
//
// Unlike the Authorization Code flow, this doesn't require a local
// redirect/callback server, so it works fine over SSH - the user can
// complete sign-in from any browser on any device.
func (t *TeamsUpdater) AuthenticateInteractive(ctx context.Context) error {
	if t.clientID == "" || t.tenantID == "" {
		return fmt.Errorf("client_id and tenant_id must be set in config before authenticating")
	}

	deviceCodeURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", t.tenantID)

	form := url.Values{}
	form.Set("client_id", t.clientID)
	form.Set("scope", teamsGraphScope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var dc struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Message         string `json:"message"`
	}
	if err := json.Unmarshal(body, &dc); err != nil {
		return fmt.Errorf("failed to parse device code response: %w", err)
	}

	// Show the user what to do. This works identically whether running
	// locally or over SSH, since it's just a URL + short code.
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Microsoft Teams sign-in required")
	fmt.Println("═══════════════════════════════════════════════════════")
	if dc.Message != "" {
		fmt.Println(dc.Message)
	} else {
		fmt.Printf("Go to %s and enter code: %s\n", dc.VerificationURI, dc.UserCode)
	}
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("Waiting for you to complete sign-in...")

	interval := dc.Interval
	if interval <= 0 {
		interval = 5
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", t.tenantID)
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		pollForm := url.Values{}
		pollForm.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		pollForm.Set("client_id", t.clientID)
		pollForm.Set("device_code", dc.DeviceCode)

		tokenResp, pollErr := t.postForToken(ctx, tokenURL, pollForm)
		if pollErr == nil {
			t.accessToken = tokenResp.AccessToken
			t.refreshToken = tokenResp.RefreshToken
			fmt.Println("✅ Signed in successfully!")
			return t.persistTokens()
		}

		// authorization_pending is expected while waiting; anything else
		// (like authorization_declined or expired_token) should abort.
		if errResp, ok := pollErr.(*deviceFlowError); ok {
			switch errResp.ErrorCode {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5
				continue
			default:
				return fmt.Errorf("sign-in failed: %s", errResp.ErrorDescription)
			}
		}

		return pollErr
	}

	return fmt.Errorf("sign-in timed out - please try again")
}

// deviceFlowError represents an OAuth device-flow-specific error response
type deviceFlowError struct {
	ErrorCode        string
	ErrorDescription string
}

func (e *deviceFlowError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrorCode, e.ErrorDescription)
}

// tokenResponse is the common shape of a successful token endpoint response
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// postForToken posts a form-encoded request to a Microsoft token endpoint
// and parses either a successful token response or an OAuth error.
func (t *TeamsUpdater) postForToken(ctx context.Context, tokenURL string, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return nil, &deviceFlowError{ErrorCode: errResp.Error, ErrorDescription: errResp.ErrorDescription}
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response did not contain an access token")
	}

	return &tr, nil
}

// persistTokens writes the updated access/refresh tokens back to the
// omniStatus config file so subsequent runs use the fresh tokens.
func (t *TeamsUpdater) persistTokens() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config for token persistence: %w", err)
	}

	if cfg.Teams == nil {
		cfg.Teams = &config.PlatformConfig{Enabled: true}
	}

	cfg.Teams.Token = t.accessToken
	if cfg.Teams.Extra == nil {
		cfg.Teams.Extra = make(map[string]string)
	}
	cfg.Teams.Extra["refresh_token"] = t.refreshToken

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	return nil
}

// mapStateToTeamsPresence converts our PresenceState to a valid Teams
// (availability, activity) pair. Microsoft Graph validates these as a
// matched pair - not all combinations are accepted, so each state maps
// to a known-good pairing (verified against the live API).
func (t *TeamsUpdater) mapStateToTeamsPresence(state PresenceState) (availability string, activity string) {
	switch state {
	case StateActive:
		return "Available", "Available"
	case StateAway:
		return "Away", "Away"
	case StateDND:
		return "DoNotDisturb", "DoNotDisturb"
	case StateBusy:
		return "Busy", "Busy"
	case StateBRB:
		return "BeRightBack", "BeRightBack"
	case StateOffline:
		return "Offline", "OffWork"
	default:
		return "Available", "Available"
	}
}
