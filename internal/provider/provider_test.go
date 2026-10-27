package provider

import "testing"

func TestCatalogLoads(t *testing.T) {
	providers := All()
	if len(providers) != 8 {
		t.Fatalf("All() returned %d providers, want 8", len(providers))
	}
}

func TestKimiCodeUsesStaticCatalogue(t *testing.T) {
	p, err := Get("kimi-code")
	if err != nil {
		t.Fatalf("Get(kimi-code) error: %v", err)
	}
	if p.BaseURL != "https://api.kimi.ai/coding/" {
		t.Errorf("BaseURL = %q", p.BaseURL)
	}
	models, err := p.Catalogue("not-used-for-static")
	if err != nil {
		t.Fatalf("Catalogue() error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "kimi-for-coding" {
		t.Fatalf("Catalogue() = %+v, want one kimi-for-coding entry", models)
	}
	if w := models[0].ContextWindow(); w != 262144 {
		t.Errorf("ContextWindow = %d, want 262144", w)
	}
}

func TestMoonshotPlatformListsModels(t *testing.T) {
	p, err := Get("moonshot")
	if err != nil {
		t.Fatalf("Get(moonshot) error: %v", err)
	}
	if p.ModelsEndpoint() != "https://api.moonshot.ai/v1/models" {
		t.Errorf("ModelsEndpoint = %q", p.ModelsEndpoint())
	}
	if len(p.Models) != 0 {
		t.Errorf("platform provider should fetch its catalogue, got %d static models", len(p.Models))
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
