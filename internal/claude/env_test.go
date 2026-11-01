package claude

import (
	"testing"

	"github.com/mutex-solutions/clavis/internal/profile"
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
	if v["ANTHROPIC_DEFAULT_FABLE_MODEL"] != "m" {
		t.Errorf("fable fallback = %q, want m", v["ANTHROPIC_DEFAULT_FABLE_MODEL"])
	}
}

func TestEnvVars_OAuth(t *testing.T) {
	// A setup-token profile authenticates against Anthropic itself. It must not
	// carry an API-key header or a base URL, and with no model bound it must not
	// pin a model either.
	p := &profile.Profile{
		Name: "acct", ProviderID: "claude",
		BaseURL:  "https://api.anthropic.com",
		AuthMode: profile.AuthOAuth,
	}
	v := EnvVars(p, "sk-ant-oat")

	if v["CLAUDE_CODE_OAUTH_TOKEN"] != "sk-ant-oat" {
		t.Errorf("oauth token = %q", v["CLAUDE_CODE_OAUTH_TOKEN"])
	}
	for _, k := range []string{"ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL"} {
		if _, ok := v[k]; ok {
			t.Errorf("%s must not be set for a setup-token profile", k)
		}
	}
}

func TestBuildEnv_StripsPreviousCredential(t *testing.T) {
	// Switching a profile to a setup token must not leave the shell's gateway
	// credential in the environment: ANTHROPIC_AUTH_TOKEN outranks the token.
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "leak")
	t.Setenv("ANTHROPIC_BASE_URL", "https://ai.dinhlongviolin1.com")

	p := &profile.Profile{
		Name: "acct", ProviderID: "claude",
		BaseURL:  "https://api.anthropic.com",
		AuthMode: profile.AuthOAuth,
	}
	env, err := buildEnv(p, "tok")
	if err != nil {
		t.Fatal(err)
	}
	m := envToMap(env)
	if v, ok := m["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Errorf("inherited ANTHROPIC_AUTH_TOKEN survived: %q", v)
	}
	if v, ok := m["ANTHROPIC_BASE_URL"]; ok {
		t.Errorf("inherited ANTHROPIC_BASE_URL survived: %q", v)
	}
	if m["CLAUDE_CODE_OAUTH_TOKEN"] != "tok" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q", m["CLAUDE_CODE_OAUTH_TOKEN"])
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
