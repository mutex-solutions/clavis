package profile

import "testing"

func TestValidatePickModeAllowsNoBoundModel(t *testing.T) {
	p := &Profile{
		Name:       "picky",
		ProviderID: "kimi-code",
		BaseURL:    "https://api.kimi.ai/coding/",
		ModelMode:  "pick",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("pick-mode profile should validate without a model: %v", err)
	}
	if !p.PicksModelAtLaunch() {
		t.Error("PicksModelAtLaunch() = false")
	}
}

func TestValidatePickModeRejectsBoundModels(t *testing.T) {
	p := &Profile{
		Name:        "mixed",
		ProviderID:  "kimi-code",
		BaseURL:     "https://api.kimi.ai/coding/",
		ModelMode:   "pick",
		SonnetModel: "kimi-for-coding",
	}
	if err := p.Validate(); err == nil {
		t.Fatal("pick mode with a bound model should fail validation")
	}
}

func TestValidateRejectsUnknownModelMode(t *testing.T) {
	p := &Profile{
		Name:        "odd",
		ProviderID:  "deepseek",
		BaseURL:     "https://api.deepseek.com/anthropic",
		ModelMode:   "whatever",
		SonnetModel: "deepseek-chat",
	}
	if err := p.Validate(); err == nil {
		t.Fatal("unknown model_mode should fail validation")
	}
}

func TestBoundProfileStillRequiresAModel(t *testing.T) {
	p := &Profile{Name: "bound", ProviderID: "deepseek", BaseURL: "https://api.deepseek.com/anthropic"}
	if err := p.Validate(); err == nil {
		t.Fatal("a profile without model_mode pick and without a model should fail")
	}
}
