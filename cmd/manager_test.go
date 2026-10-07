package cmd

import (
	"strings"
	"testing"

	"github.com/SinisterNerd/omnistatus/config"
)

func TestBuildManagerPlatformFilter(t *testing.T) {
	cfg := &config.Config{
		Slack: &config.PlatformConfig{Enabled: true, Token: "a"},
		Instances: map[string]*config.PlatformConfig{
			"slack-work": {Type: "slack", Enabled: true, Token: "b"},
			"gh-off":     {Type: "github", Enabled: false, Token: "c"},
		},
	}

	// Naming only slack-work must not try Teams sign-in (cfg.Teams is nil
	// here; reaching getTeamsUpdater would hang on interactive auth).
	if _, err := buildManager(cfg, []string{"slack-work", " slack "}); err != nil {
		t.Fatalf("valid filter: %v", err)
	}

	for _, bad := range []string{"nope", "gh-off", "teams"} {
		_, err := buildManager(cfg, []string{bad})
		if err == nil || !strings.Contains(err.Error(), bad) {
			t.Errorf("%q: expected not-enabled error, got %v", bad, err)
		}
	}
}
