package claude

import (
	"os"
	"strings"
	"testing"

	"github.com/dinhlongviolin1/clavis/internal/profile"
)

func envToMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func TestBuildEnvCoreVars(t *testing.T) {
	p := &profile.Profile{
		Name:          "test",
		ProviderID:    "deepseek",
		BaseURL:       "https://api.deepseek.com/anthropic",
		SonnetModel:   "deepseek-v4-pro",
		OpusModel:     "deepseek-v4-pro",
		HaikuModel:    "deepseek-v4-flash",
		SubagentModel: "deepseek-v4-flash",
	}

	env, err := buildEnv(p, "sk-test-key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)

	checks := map[string]string{
		"ANTHROPIC_BASE_URL":             "https://api.deepseek.com/anthropic",
		"ANTHROPIC_AUTH_TOKEN":           "sk-test-key",
		"ANTHROPIC_MODEL":                "deepseek-v4-pro",
		"ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-pro",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   "deepseek-v4-pro",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "deepseek-v4-flash",
		"CLAUDE_CODE_SUBAGENT_MODEL":     "deepseek-v4-flash",
	}

	for key, want := range checks {
		if got := m[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestBuildEnvUnsetsAPIKey(t *testing.T) {
	// Simulate user having ANTHROPIC_API_KEY in their shell
	t.Setenv("ANTHROPIC_API_KEY", "sk-user-anthropic-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token")

	p := &profile.Profile{
		Name:        "test",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "model",
	}

	env, err := buildEnv(p, "sk-3p-key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)

	if _, exists := m["ANTHROPIC_API_KEY"]; exists {
		t.Error("ANTHROPIC_API_KEY should be unset, but found in env")
	}
	if _, exists := m["CLAUDE_CODE_OAUTH_TOKEN"]; exists {
		t.Error("CLAUDE_CODE_OAUTH_TOKEN should be unset, but found in env")
	}
}

func TestBuildEnvNoDuplicates(t *testing.T) {
	// Simulate user having ANTHROPIC_BASE_URL in their shell
	t.Setenv("ANTHROPIC_BASE_URL", "https://user-override.example.com")

	p := &profile.Profile{
		Name:        "test",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "model",
	}

	env, err := buildEnv(p, "key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	// Count occurrences of ANTHROPIC_BASE_URL
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_BASE_URL=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ANTHROPIC_BASE_URL appears %d times, want exactly 1", count)
	}

	// Verify it's the profile value, not the shell value
	m := envToMap(env)
	if got := m["ANTHROPIC_BASE_URL"]; got != "https://api.deepseek.com/anthropic" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want profile value", got)
	}
}

func TestBuildEnvHygieneDefaults(t *testing.T) {
	// 3P provider with nil *bool → should set both to "1"
	p := &profile.Profile{
		Name:        "test",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "model",
	}

	env, err := buildEnv(p, "key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)

	if got := m["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"]; got != "1" {
		t.Errorf("3P NonessentialTraffic = %q, want '1'", got)
	}
	if got := m["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"]; got != "1" {
		t.Errorf("3P ExperimentalBetas = %q, want '1'", got)
	}
}

func TestBuildEnvAnthropicNoHygiene(t *testing.T) {
	// Anthropic with nil *bool → should NOT set hygiene vars
	p := &profile.Profile{
		Name:        "test",
		ProviderID:  "anthropic",
		BaseURL:     "https://api.anthropic.com",
		SonnetModel: "claude-sonnet-5",
	}

	env, err := buildEnv(p, "key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)

	if _, exists := m["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"]; exists {
		t.Error("anthropic should not set NonessentialTraffic by default")
	}
	if _, exists := m["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"]; exists {
		t.Error("anthropic should not set ExperimentalBetas by default")
	}
}

func TestBuildEnvOptionalFields(t *testing.T) {
	compactPct := 80
	contextWindow := 128000
	maxOutput := 16384
	timeout := 3000000

	p := &profile.Profile{
		Name:            "test",
		ProviderID:      "deepseek",
		BaseURL:         "https://api.deepseek.com/anthropic",
		SonnetModel:     "model",
		AutoCompactPct:  &compactPct,
		ContextWindow:   &contextWindow,
		MaxOutputTokens: &maxOutput,
		TimeoutMs:       &timeout,
		EffortLevel:     "high",
		DisableThinking: true,
	}

	env, err := buildEnv(p, "key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)

	if got := m["CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"]; got != "80" {
		t.Errorf("AutoCompactPct = %q, want '80'", got)
	}
	if got := m["CLAUDE_CODE_AUTO_COMPACT_WINDOW"]; got != "128000" {
		t.Errorf("ContextWindow = %q, want '128000'", got)
	}
	if got := m["CLAUDE_CODE_MAX_OUTPUT_TOKENS"]; got != "16384" {
		t.Errorf("MaxOutputTokens = %q, want '16384'", got)
	}
	if got := m["API_TIMEOUT_MS"]; got != "3000000" {
		t.Errorf("TimeoutMs = %q, want '3000000'", got)
	}
	if got := m["CLAUDE_CODE_EFFORT_LEVEL"]; got != "high" {
		t.Errorf("EffortLevel = %q, want 'high'", got)
	}
	if got := m["CLAUDE_CODE_DISABLE_THINKING"]; got != "1" {
		t.Errorf("DisableThinking = %q, want '1'", got)
	}
}

func TestBuildEnvConfigDir(t *testing.T) {
	p := &profile.Profile{
		Name:        "my-profile",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "model",
	}

	env, err := buildEnv(p, "key")
	if err != nil {
		t.Fatalf("buildEnv() error: %v", err)
	}

	m := envToMap(env)
	configDir := m["CLAUDE_CONFIG_DIR"]
	if !strings.HasSuffix(configDir, ".clavis/profiles/my-profile") {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want suffix .clavis/profiles/my-profile", configDir)
	}
}

func TestBuildArgsMinimalMode(t *testing.T) {
	p := &profile.Profile{MinimalMode: true}
	args := buildArgs(p, []string{"-p", "hello"})

	if args[0] != "--bare" {
		t.Errorf("first arg = %q, want --bare", args[0])
	}
	if len(args) != 3 {
		t.Errorf("args count = %d, want 3", len(args))
	}
}

func TestBuildArgsNoMinimal(t *testing.T) {
	p := &profile.Profile{MinimalMode: false}
	args := buildArgs(p, []string{"-p", "hello"})

	if len(args) != 2 {
		t.Errorf("args count = %d, want 2 (no --bare)", len(args))
	}
}

func TestWriteSettings(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	p := &profile.Profile{
		Name:        "settings-test",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "model",
	}

	err := writeSettings(p)
	if err != nil {
		t.Fatalf("writeSettings() error: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(tmpDir + "/.clavis/profiles/settings-test/settings.json")
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "CLAUDE_CODE_ATTRIBUTION_HEADER") {
		t.Error("settings.json should contain CLAUDE_CODE_ATTRIBUTION_HEADER")
	}
	if !strings.Contains(content, `"0"`) {
		t.Error("attribution header should be '0'")
	}
}
