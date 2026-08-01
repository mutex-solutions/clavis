package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"deepseek", false},
		{"work-profile", false},
		{"a", false},
		{"", true},
		{".", true},
		{"..", true},
		{"../evil", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{"a..b", true},
		{"../../etc/passwd", true},
	}

	for _, tt := range tests {
		err := ValidName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestTierFallback(t *testing.T) {
	p := &Profile{
		SonnetModel: "model-sonnet",
		OpusModel:   "",
		HaikuModel:  "",
		SubagentModel: "",
	}

	if got := p.ResolvedOpus(); got != "model-sonnet" {
		t.Errorf("ResolvedOpus() = %q, want %q", got, "model-sonnet")
	}
	if got := p.ResolvedHaiku(); got != "model-sonnet" {
		t.Errorf("ResolvedHaiku() = %q, want %q", got, "model-sonnet")
	}
	if got := p.ResolvedSubagent(); got != "model-sonnet" {
		t.Errorf("ResolvedSubagent() = %q, want %q", got, "model-sonnet")
	}

	// With explicit values
	p.OpusModel = "model-opus"
	p.HaikuModel = "model-haiku"
	p.SubagentModel = "model-sub"

	if got := p.ResolvedOpus(); got != "model-opus" {
		t.Errorf("ResolvedOpus() = %q, want %q", got, "model-opus")
	}
	if got := p.ResolvedHaiku(); got != "model-haiku" {
		t.Errorf("ResolvedHaiku() = %q, want %q", got, "model-haiku")
	}
	if got := p.ResolvedSubagent(); got != "model-sub" {
		t.Errorf("ResolvedSubagent() = %q, want %q", got, "model-sub")
	}
}

func TestHygieneDefaults(t *testing.T) {
	// 3P provider, nil *bool → should default to true
	p3p := &Profile{ProviderID: "deepseek"}
	if !p3p.ShouldDisableNonessentialTraffic() {
		t.Error("3P provider should default DisableNonessentialTraffic=true")
	}
	if !p3p.ShouldDisableExperimentalBetas() {
		t.Error("3P provider should default DisableExperimentalBetas=true")
	}

	// Anthropic, nil *bool → should default to false
	pAnth := &Profile{ProviderID: "anthropic"}
	if pAnth.ShouldDisableNonessentialTraffic() {
		t.Error("anthropic should default DisableNonessentialTraffic=false")
	}
	if pAnth.ShouldDisableExperimentalBetas() {
		t.Error("anthropic should default DisableExperimentalBetas=false")
	}

	// Explicit override
	f := false
	tr := true
	p3p.DisableNonessentialTraffic = &f
	if p3p.ShouldDisableNonessentialTraffic() {
		t.Error("explicit false should override 3P default")
	}
	pAnth.DisableExperimentalBetas = &tr
	if !pAnth.ShouldDisableExperimentalBetas() {
		t.Error("explicit true should override anthropic default")
	}
}

func TestSaveLoadDelete(t *testing.T) {
	// Use a temp dir
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origDir)

	p := &Profile{
		Name:        "test-profile",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		SonnetModel: "deepseek-v4-pro",
		OpusModel:   "deepseek-v4-pro",
		HaikuModel:  "deepseek-v4-flash",
	}

	// Save
	err := Save(p, "sk-test-key-123")
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify .env permissions
	envPath := filepath.Join(ProfileDir("test-profile"), ".env")
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("stat .env: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf(".env permissions = %o, want 0600", info.Mode().Perm())
	}

	// Load
	loaded, err := Load("test-profile")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.SonnetModel != "deepseek-v4-pro" {
		t.Errorf("SonnetModel = %q, want %q", loaded.SonnetModel, "deepseek-v4-pro")
	}
	if loaded.ProviderID != "deepseek" {
		t.Errorf("ProviderID = %q, want %q", loaded.ProviderID, "deepseek")
	}
	if loaded.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}

	// LoadKey
	key, err := LoadKey("test-profile")
	if err != nil {
		t.Fatalf("LoadKey() error: %v", err)
	}
	if key != "sk-test-key-123" {
		t.Errorf("LoadKey() = %q, want %q", key, "sk-test-key-123")
	}

	// Exists
	if !Exists("test-profile") {
		t.Error("Exists() should return true")
	}
	if Exists("nonexistent") {
		t.Error("Exists() should return false for nonexistent")
	}

	// List
	profiles, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("List() returned %d profiles, want 1", len(profiles))
	}

	// Delete
	err = Delete("test-profile")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if Exists("test-profile") {
		t.Error("profile should not exist after delete")
	}

	// Delete nonexistent
	err = Delete("nonexistent")
	if err == nil {
		t.Error("Delete(nonexistent) should return error")
	}
}

func TestSaveRejectsTraversal(t *testing.T) {
	p := &Profile{
		Name:        "../../etc/evil",
		ProviderID:  "deepseek",
		BaseURL:     "https://example.com",
		SonnetModel: "model",
	}
	err := Save(p, "key")
	if err == nil {
		t.Fatal("Save() should reject path traversal in name")
	}
}

func TestLoadRejectsTraversal(t *testing.T) {
	_, err := Load("../../../etc/passwd")
	if err == nil {
		t.Fatal("Load() should reject path traversal")
	}
}

func TestStateLastUsed(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origDir)

	// Initially empty
	if got := LoadLastUsed(); got != "" {
		t.Errorf("LoadLastUsed() = %q, want empty", got)
	}

	// Save and reload
	SaveLastUsed("my-profile")
	if got := LoadLastUsed(); got != "my-profile" {
		t.Errorf("LoadLastUsed() = %q, want %q", got, "my-profile")
	}

	// Overwrite
	SaveLastUsed("other-profile")
	if got := LoadLastUsed(); got != "other-profile" {
		t.Errorf("LoadLastUsed() = %q, want %q", got, "other-profile")
	}
}
