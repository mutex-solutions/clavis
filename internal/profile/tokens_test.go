package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSavePreservesKeyWhenEmpty is what makes `clavis edit` safe: the editor
// never handles the stored secret, so it saves with an empty key and the .env
// on disk must come through byte-for-byte identical.
func TestSavePreservesKeyWhenEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	p := &Profile{
		Name:        "editme",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "deepseek-v4-pro",
	}
	if err := Save(p, "sk-original-key"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	envPath := filepath.Join(ProfileDir("editme"), ".env")
	before, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}

	// Change an unrelated setting the way the editor does.
	window := 128000
	p.MaxContextTokens = &window
	if err := Save(p, ""); err != nil {
		t.Fatalf("Save() with an empty key error: %v", err)
	}

	after, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env after save: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf(".env changed on a keyless save:\n before %q\n after  %q", before, after)
	}

	key, err := LoadKey("editme")
	if err != nil {
		t.Fatalf("LoadKey() error: %v", err)
	}
	if key != "sk-original-key" {
		t.Errorf("LoadKey() = %q, want the preserved %q", key, "sk-original-key")
	}
	if info, err := os.Stat(envPath); err == nil && info.Mode().Perm() != 0o600 {
		t.Errorf(".env permissions = %o, want 0600", info.Mode().Perm())
	}
}

// TestTokenBudgetRoundTrip covers the three knobs surviving the JSON round trip,
// and — just as important — staying absent when unset, so an edited profile
// doesn't grow explicit zeros that would override Claude Code's own defaults.
func TestTokenBudgetRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	maxContext, compactPct, compactWindow := 1000000, 75, 900000
	p := &Profile{
		Name:              "tok",
		ProviderID:        "custom",
		BaseURL:           "https://api.example.com/anthropic",
		SonnetModel:       "big-model",
		MaxContextTokens:  &maxContext,
		AutoCompactPct:    &compactPct,
		AutoCompactWindow: &compactWindow,
	}
	if err := Save(p, "k"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load("tok")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.MaxContextTokens == nil || *loaded.MaxContextTokens != 1000000 {
		t.Errorf("MaxContextTokens = %v, want 1000000", loaded.MaxContextTokens)
	}
	if loaded.AutoCompactPct == nil || *loaded.AutoCompactPct != 75 {
		t.Errorf("AutoCompactPct = %v, want 75", loaded.AutoCompactPct)
	}
	if loaded.AutoCompactWindow == nil || *loaded.AutoCompactWindow != 900000 {
		t.Errorf("AutoCompactWindow = %v, want 900000", loaded.AutoCompactWindow)
	}

	// The old key must be gone: a stray context_window would silently reassert
	// the pre-rename meaning of the field.
	raw, err := os.ReadFile(filepath.Join(ProfileDir("tok"), "profile.json"))
	if err != nil {
		t.Fatalf("read profile.json: %v", err)
	}
	if got := string(raw); strings.Contains(got, `"context_window"`) {
		t.Errorf("profile.json still carries the old context_window key:\n%s", got)
	}

	// Unset knobs stay out of the file entirely.
	bare := &Profile{
		Name:        "bare",
		ProviderID:  "custom",
		BaseURL:     "https://api.example.com/anthropic",
		SonnetModel: "m",
	}
	if err := Save(bare, "k"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	raw, err = os.ReadFile(filepath.Join(ProfileDir("bare"), "profile.json"))
	if err != nil {
		t.Fatalf("read profile.json: %v", err)
	}
	for _, key := range []string{"max_context_tokens", "auto_compact_pct", "auto_compact_window"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("unset %s was written to profile.json:\n%s", key, raw)
		}
	}
}
