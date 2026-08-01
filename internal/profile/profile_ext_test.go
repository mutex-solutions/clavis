package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCorruptProfileHiddenInList(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	// Create a valid profile
	dir := ProfileDir("good")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"name":"good","provider_id":"deepseek","base_url":"https://example.com","sonnet_model":"model"}`), 0644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("CLAVIS_KEY=sk-test\n"), 0600)

	// Create a corrupt profile
	corruptDir := ProfileDir("corrupt")
	os.MkdirAll(corruptDir, 0755)
	os.WriteFile(filepath.Join(corruptDir, "profile.json"), []byte(`not json`), 0644)
	os.WriteFile(filepath.Join(corruptDir, ".env"), []byte("CLAVIS_KEY=sk-test\n"), 0600)

	profiles, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(profiles) != 1 {
		t.Errorf("List() returned %d profiles, want 1 (corrupt should be skipped)", len(profiles))
	}
	if profiles[0].Name != "good" {
		t.Errorf("Expected 'good' profile, got %q", profiles[0].Name)
	}
}

func TestSaveValidatesProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	// Missing SonnetModel
	p := &Profile{
		Name:       "incomplete",
		ProviderID: "deepseek",
		BaseURL:    "https://example.com",
	}
	err := Save(p, "key")
	if err == nil {
		t.Fatal("Save() should reject profile without SonnetModel")
	}
}

func TestProfileNameMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	dir := ProfileDir("foo")
	os.MkdirAll(dir, 0755)
	// profile.json says name is "bar" but directory is "foo"
	os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"name":"bar","provider_id":"deepseek","base_url":"https://example.com","sonnet_model":"model"}`), 0644)

	_, err := Load("foo")
	if err == nil {
		t.Fatal("Load() should reject profile with mismatched internal name")
	}
}

func TestValidNameRejectsReservedWords(t *testing.T) {
	reserved := []string{"add", "list", "ls", "remove", "rm", "start", "providers", "validate", "version", "help"}
	for _, name := range reserved {
		err := ValidName(name)
		if err == nil {
			t.Errorf("ValidName(%q) should reject reserved command name", name)
		}
	}
}

func TestValidNameRejectsLeadingDash(t *testing.T) {
	for _, name := range []string{"-evil", "--double-dash"} {
		err := ValidName(name)
		if err == nil {
			t.Errorf("ValidName(%q) should reject leading dash", name)
		}
	}
}

func TestValidNameRejectsControlChars(t *testing.T) {
	for _, name := range []string{"foo\nbar", "foo\tbar", "foo\rbar"} {
		err := ValidName(name)
		if err == nil {
			t.Errorf("ValidName(%q) should reject control characters", name)
		}
	}
}

func TestValidateEmptyFields(t *testing.T) {
	tests := []struct {
		name string
		p    Profile
	}{
		{"no sonnet", Profile{Name: "x", ProviderID: "p", BaseURL: "https://x.com", SonnetModel: ""}},
		{"no provider", Profile{Name: "x", ProviderID: "", BaseURL: "https://x.com", SonnetModel: "m"}},
		{"no baseURL", Profile{Name: "x", ProviderID: "p", BaseURL: "", SonnetModel: "m"}},
		{"no name", Profile{Name: "", ProviderID: "p", BaseURL: "https://x.com", SonnetModel: "m"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Validate(); err == nil {
				t.Error("Validate() should error")
			}
		})
	}
}
