package provider

import "testing"

func TestCatalogLoads(t *testing.T) {
	providers := All()
	if len(providers) != 8 {
		t.Fatalf("All() returned %d providers, want 8", len(providers))
	}
}

func TestGet(t *testing.T) {
	p, err := Get("deepseek")
	if err != nil {
		t.Fatalf("Get(deepseek) error: %v", err)
	}
	if p.BaseURL != "https://api.deepseek.com/anthropic" {
		t.Errorf("BaseURL = %q", p.BaseURL)
	}

	// Nonexistent
	_, err = Get("nonexistent")
	if err == nil {
		t.Error("Get(nonexistent) should return error")
	}
}

func TestCustomProviderEmptyBaseURL(t *testing.T) {
	p, err := Get("custom")
	if err != nil {
		t.Fatalf("Get(custom) error: %v", err)
	}
	if p.BaseURL != "" {
		t.Errorf("custom provider BaseURL should be empty, got %q", p.BaseURL)
	}
}

func TestAllProvidersHaveAuth(t *testing.T) {
	for _, p := range All() {
		if p.Auth.EnvVar == "" {
			t.Errorf("provider %q has empty Auth.EnvVar", p.ID)
		}
	}
}
