package claude

import (
	"testing"

	"github.com/dinhlongviolin1/clavis/internal/profile"
)

func TestEnvVars_SharedByDefault(t *testing.T) {
	p := &profile.Profile{
		Name: "x", ProviderID: "deepseek",
		BaseURL: "https://api.deepseek.com/anthropic", SonnetModel: "m",
	}
	v := EnvVars(p, "sk-key")

	if v["ANTHROPIC_AUTH_TOKEN"] != "sk-key" {
		t.Errorf("auth token = %q", v["ANTHROPIC_AUTH_TOKEN"])
	}
	if v["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("base url = %q", v["ANTHROPIC_BASE_URL"])
	}
	// Shared mode: CLAUDE_CONFIG_DIR must NOT be set.
	if _, ok := v["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("shared profile should not set CLAUDE_CONFIG_DIR")
	}
	// Tier fallbacks resolve to sonnet.
	if v["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "m" {
		t.Errorf("opus fallback = %q, want m", v["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	}
}

func TestEnvVars_Isolated(t *testing.T) {
	p := &profile.Profile{
		Name: "x", ProviderID: "deepseek",
		BaseURL: "https://api.deepseek.com/anthropic", SonnetModel: "m",
		IsolatedConfig: true,
	}
	v := EnvVars(p, "sk-key")
	if v["CLAUDE_CONFIG_DIR"] == "" {
		t.Error("isolated profile must set CLAUDE_CONFIG_DIR")
	}
}

func TestEnvVars_MatchesBuildEnv(t *testing.T) {
	// Every var EnvVars returns must appear in the full buildEnv output — they
	// share one source of truth.
	p := &profile.Profile{
		Name: "x", ProviderID: "deepseek",
		BaseURL: "https://api.deepseek.com/anthropic", SonnetModel: "m",
		EffortLevel: "high", IsolatedConfig: true,
	}
	set := EnvVars(p, "sk-key")
	full, err := buildEnv(p, "sk-key")
	if err != nil {
		t.Fatal(err)
	}
	fm := envToMap(full)
	for k, want := range set {
		if got := fm[k]; got != want {
			t.Errorf("buildEnv[%s] = %q, EnvVars = %q", k, got, want)
		}
	}
}
