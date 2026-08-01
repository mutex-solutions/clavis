package provider

import "testing"

func TestCatalogLoads(t *testing.T) {
	providers := All()
	if len(providers) != 7 {
		t.Fatalf("All() returned %d providers, want 7", len(providers))
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
	if p.Tier != "paid" {
		t.Errorf("Tier = %q, want paid", p.Tier)
	}
	if len(p.Models) != 2 {
		t.Errorf("Models count = %d, want 2", len(p.Models))
	}

	// Nonexistent
	_, err = Get("nonexistent")
	if err == nil {
		t.Error("Get(nonexistent) should return error")
	}
}

func TestModelsForTier(t *testing.T) {
	p, _ := Get("deepseek")

	sonnetModels := p.ModelsForTier("sonnet")
	if len(sonnetModels) != 1 {
		t.Fatalf("ModelsForTier(sonnet) = %d models, want 1", len(sonnetModels))
	}
	if sonnetModels[0].ID != "deepseek-v4-pro" {
		t.Errorf("sonnet model = %q, want deepseek-v4-pro", sonnetModels[0].ID)
	}

	haikuModels := p.ModelsForTier("haiku")
	if len(haikuModels) != 1 {
		t.Fatalf("ModelsForTier(haiku) = %d models, want 1", len(haikuModels))
	}
	if haikuModels[0].ID != "deepseek-v4-flash" {
		t.Errorf("haiku model = %q, want deepseek-v4-flash", haikuModels[0].ID)
	}

	opusModels := p.ModelsForTier("opus")
	if len(opusModels) != 1 {
		t.Fatalf("ModelsForTier(opus) = %d models, want 1", len(opusModels))
	}
}

func TestDefaultForTier(t *testing.T) {
	p, _ := Get("deepseek")

	if got := p.DefaultForTier("sonnet"); got != "deepseek-v4-pro" {
		t.Errorf("DefaultForTier(sonnet) = %q, want deepseek-v4-pro", got)
	}
	if got := p.DefaultForTier("haiku"); got != "deepseek-v4-flash" {
		t.Errorf("DefaultForTier(haiku) = %q, want deepseek-v4-flash", got)
	}
	if got := p.DefaultForTier("nonexistent"); got != "" {
		t.Errorf("DefaultForTier(nonexistent) = %q, want empty", got)
	}
}

func TestCustomProviderHasNoModels(t *testing.T) {
	p, err := Get("custom")
	if err != nil {
		t.Fatalf("Get(custom) error: %v", err)
	}
	if len(p.Models) != 0 {
		t.Errorf("custom provider should have 0 models, got %d", len(p.Models))
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

func TestTierHintsNotEmpty(t *testing.T) {
	for _, p := range All() {
		for _, m := range p.Models {
			if len(m.TierHints) == 0 {
				t.Errorf("model %q in provider %q has no tierHints", m.ID, p.ID)
			}
		}
	}
}
