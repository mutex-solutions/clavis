package provider

import "testing"

func TestCatalogLoads(t *testing.T) {
	providers := All()
	if len(providers) != 8 {
		t.Fatalf("All() returned %d providers, want 8", len(providers))
	}
}

func TestKimiCodeListsModels(t *testing.T) {
	p, err := Get("kimi-code")
	if err != nil {
		t.Fatalf("Get(kimi-code) error: %v", err)
	}
	if p.BaseURL != "https://api.kimi.ai/coding/" {
		t.Errorf("BaseURL = %q", p.BaseURL)
	}
	// The coding endpoint does expose a listing (measured 2026-10-27, and it
	// reports a 1M window rather than the 256K the docs suggested), so the
	// catalog must ask it instead of hardcoding a stale number.
	if p.ModelsEndpoint() != "https://api.kimi.ai/coding/v1/models" {
		t.Errorf("ModelsEndpoint = %q", p.ModelsEndpoint())
	}
	if len(p.Models) != 0 {
		t.Errorf("kimi-code should use the live listing, got %d static models", len(p.Models))
	}
}

func TestStaticCatalogueSkipsTheNetwork(t *testing.T) {
	window := 131072
	p := Provider{
		BaseURL: "http://127.0.0.1:1/never-reached",
		Models:  []ModelInfo{{ID: "fixed-model", MaxInputTokens: &window}},
	}
	models, err := p.Catalogue("unused")
	if err != nil {
		t.Fatalf("Catalogue() error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "fixed-model" {
		t.Fatalf("Catalogue() = %+v, want one fixed-model entry", models)
	}
	if got := models[0].ContextWindow(); got != window {
		t.Errorf("ContextWindow = %d, want %d", got, window)
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
