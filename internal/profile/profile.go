package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Profile is a config pack: provider routing + model tiers + token management.
type Profile struct {
	// Identity
	Name       string `json:"name"`
	ProviderID string `json:"provider_id"`
	BaseURL    string `json:"base_url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`

	// Model Tiers (sonnet required, others optional → fall back to sonnet)
	SonnetModel   string `json:"sonnet_model"`
	OpusModel     string `json:"opus_model,omitempty"`
	HaikuModel    string `json:"haiku_model,omitempty"`
	SubagentModel string `json:"subagent_model,omitempty"`

	// Token Budget
	ContextWindow   *int   `json:"context_window,omitempty"`
	AutoCompactPct  *int   `json:"auto_compact_pct,omitempty"`
	MaxOutputTokens *int   `json:"max_output_tokens,omitempty"`
	EffortLevel     string `json:"effort_level,omitempty"`
	DisableThinking bool   `json:"disable_thinking,omitempty"`

	// Resilience
	TimeoutMs *int `json:"timeout_ms,omitempty"`

	// 3P Provider Hygiene (*bool: nil = provider default)
	DisableNonessentialTraffic *bool `json:"disable_nonessential_traffic,omitempty"`
	DisableExperimentalBetas   *bool `json:"disable_experimental_betas,omitempty"`
	DisableAttribution         *bool `json:"disable_attribution,omitempty"`

	// Launch Preferences
	MinimalMode   bool              `json:"minimal_mode,omitempty"`
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
}

// ResolvedOpus returns opus model or falls back to sonnet.
func (p *Profile) ResolvedOpus() string {
	if p.OpusModel != "" {
		return p.OpusModel
	}
	return p.SonnetModel
}

// ResolvedHaiku returns haiku model or falls back to sonnet.
func (p *Profile) ResolvedHaiku() string {
	if p.HaikuModel != "" {
		return p.HaikuModel
	}
	return p.SonnetModel
}

// ResolvedSubagent returns subagent model or falls back to sonnet.
func (p *Profile) ResolvedSubagent() string {
	if p.SubagentModel != "" {
		return p.SubagentModel
	}
	return p.SonnetModel
}

// ShouldDisableNonessentialTraffic returns the effective value.
// nil → true for 3P providers, false for anthropic.
func (p *Profile) ShouldDisableNonessentialTraffic() bool {
	if p.DisableNonessentialTraffic != nil {
		return *p.DisableNonessentialTraffic
	}
	return p.ProviderID != "anthropic" // default true for 3P
}

// ShouldDisableExperimentalBetas returns the effective value.
func (p *Profile) ShouldDisableExperimentalBetas() bool {
	if p.DisableExperimentalBetas != nil {
		return *p.DisableExperimentalBetas
	}
	return p.ProviderID != "anthropic"
}

// ShouldDisableAttribution returns the effective value.
func (p *Profile) ShouldDisableAttribution() bool {
	if p.DisableAttribution != nil {
		return *p.DisableAttribution
	}
	return false
}

// Validate checks profile invariants that prevent silent launch failures.
func (p *Profile) Validate() error {
	if err := ValidName(p.Name); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	if p.ProviderID == "" {
		return fmt.Errorf("provider_id is required")
	}
	if p.SonnetModel == "" {
		return fmt.Errorf("sonnet_model is required")
	}
	if p.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	return nil
}

// Reserved profile names that conflict with clavis subcommands.
var reserved = map[string]bool{
	"add": true, "list": true, "ls": true, "remove": true, "rm": true,
	"start": true, "providers": true, "validate": true, "version": true,
	"help": true, "switch": true, "edit": true, "info": true, "env": true,
}

// ValidName checks if a profile name is safe for use as a directory name.
func ValidName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if reserved[name] {
		return fmt.Errorf("name %q is reserved (conflicts with a clavis command)", name)
	}
	if strings.ContainsAny(name, "/\\\t\n\r\b") {
		return fmt.Errorf("name must not contain path separators or control characters")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("name must not contain '..'")
	}
	if name == "." {
		return fmt.Errorf("invalid name")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("name must not start with '-'")
	}
	return nil
}

// --- Storage ---

// Dir returns the clavis root directory (~/.clavis).
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback: this should never happen on a normal system.
		return filepath.Join(".", ".clavis")
	}
	return filepath.Join(home, ".clavis")
}

// ProfileDir returns the directory for a named profile.
func ProfileDir(name string) string {
	return filepath.Join(Dir(), "profiles", name)
}

// Load reads a profile by name from disk.
func Load(name string) (*Profile, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	dir := ProfileDir(name)
	data, err := os.ReadFile(filepath.Join(dir, "profile.json"))
	if err != nil {
		return nil, fmt.Errorf("profile %q not found: %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile %q is corrupt: %w", name, err)
	}
	// Validate the name inside the JSON matches what was requested
	// (prevents a corrupted profile.json from redirecting paths).
	if p.Name != name {
		return nil, fmt.Errorf("profile %q has mismatched internal name %q", name, p.Name)
	}
	if err := ValidName(p.Name); err != nil {
		return nil, fmt.Errorf("profile %q has invalid internal name: %w", name, err)
	}
	return &p, nil
}

// LoadKey reads the API key from the profile's .env file.
func LoadKey(name string) (string, error) {
	envPath := filepath.Join(ProfileDir(name), ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", fmt.Errorf("no API key for profile %q: %w", name, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CLAVIS_KEY=") {
			return strings.TrimPrefix(line, "CLAVIS_KEY="), nil
		}
	}
	return "", fmt.Errorf("CLAVIS_KEY not found in %s", envPath)
}

// Save writes a profile and its API key to disk.
// Uses atomic write (temp file + rename) to prevent corruption on disk full.
func Save(p *Profile, apiKey string) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	// Normalize base URL: strip trailing slashes.
	p.BaseURL = strings.TrimRight(p.BaseURL, "/")

	dir := ProfileDir(p.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create profile dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	// Write .env first (the critical secret), atomically.
	if apiKey != "" {
		envPath := filepath.Join(dir, ".env")
		envContent := fmt.Sprintf("CLAVIS_KEY=%s\n", apiKey)
		if err := atomicWrite(envPath, []byte(envContent), 0o600); err != nil {
			return err
		}
	}

	// Write profile.json atomically (no key).
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "profile.json"), append(data, '\n'), 0o644)
}

// atomicWrite writes data to a temp file then renames it into place.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".clavis-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // clean up on failure

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Delete removes a profile from disk.
func Delete(name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	dir := ProfileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q not found", name)
	}
	return os.RemoveAll(dir)
}

// List returns all profile names sorted alphabetically.
func List() ([]*Profile, error) {
	profilesDir := filepath.Join(Dir(), "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no profiles yet
		}
		return nil, err
	}

	var profiles []*Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := Load(e.Name())
		if err != nil {
			continue // skip corrupt profiles
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

// Exists checks if a profile exists.
func Exists(name string) bool {
	_, err := os.Stat(filepath.Join(ProfileDir(name), "profile.json"))
	return err == nil
}

// --- State (last-used) ---

type state struct {
	LastUsed string `json:"last_used"`
}

func statePath() string {
	return filepath.Join(Dir(), "state.json")
}

// LoadLastUsed returns the last-used profile name.
func LoadLastUsed() string {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return ""
	}
	var s state
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return s.LastUsed
}

// SaveLastUsed records the last-used profile name.
func SaveLastUsed(name string) {
	_ = os.MkdirAll(Dir(), 0o755)
	s := state{LastUsed: name}
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(statePath(), append(data, '\n'), 0o644)
}
