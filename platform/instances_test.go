package platform

import (
	"strings"
	"testing"

	"github.com/SinisterNerd/omnistatus/config"
)

func TestNewInstanceUpdaters(t *testing.T) {
	cfg := &config.Config{Instances: map[string]*config.PlatformConfig{
		"slack-work": {Type: "slack", Enabled: true, Token: "xoxp-a"},
		"gh-home":    {Type: "github", Enabled: true, Token: "ghp_b"},
	}}
	got, err := NewInstanceUpdaters(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name() != "gh-home" || got[0].Type() != "github" ||
		got[1].Name() != "slack-work" || got[1].Type() != "slack" {
		t.Fatalf("unexpected updaters: %+v", got)
	}
	for _, u := range got {
		if !u.IsEnabled() {
			t.Errorf("%s should be enabled", u.Name())
		}
	}
}

func TestNewInstanceUpdatersErrors(t *testing.T) {
	cases := map[string]*config.PlatformConfig{
		"slack": {Type: "slack"}, // reserved name
		"x":     {Type: "teams"}, // unsupported type
		"y":     {},              // missing type
		"z":     nil,             // empty block
	}
	for name, block := range cases {
		cfg := &config.Config{Instances: map[string]*config.PlatformConfig{name: block}}
		if _, err := NewInstanceUpdaters(cfg); err == nil || !strings.Contains(err.Error(), "instances."+name) {
			t.Errorf("%s: expected error naming the instance, got %v", name, err)
		}
	}
}

func TestDefaultIcon(t *testing.T) {
	for in, want := range map[string]string{"teams": "T", "slack-work": "Sw", "slack_personal": "Sp", "github-": "G", "": "?"} {
		if got := DefaultIcon(in); got != want {
			t.Errorf("DefaultIcon(%q) = %q, want %q", in, got, want)
		}
	}
}
