package main

import (
	"testing"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/ui"
)

func window(v int) *int { return &v }

func TestApplyLaunchSelectionBindsEveryTier(t *testing.T) {
	p := &profile.Profile{
		Name:       "default",
		ProviderID: "custom",
		BaseURL:    "https://ai.dinhlongviolin1.com",
		ModelMode:  "pick",
	}
	sel := &ui.LaunchSelection{
		Opus:       "deepseek-v4-pro",
		Sonnet:     "deepseek-flash",
		Haiku:      "deepseek-flash",
		Subagent:   "deepseek-v4-pro",
		MaxContext: window(1048576),
	}

	applyLaunchSelection(p, sel)

	if p.ModelMode != "" {
		t.Errorf("ModelMode = %q, want empty for the bound launch copy", p.ModelMode)
	}
	if p.SonnetModel != "deepseek-flash" || p.OpusModel != "deepseek-v4-pro" ||
		p.HaikuModel != "deepseek-flash" || p.SubagentModel != "deepseek-v4-pro" {
		t.Errorf("tiers = %q/%q/%q/%q", p.SonnetModel, p.OpusModel, p.HaikuModel, p.SubagentModel)
	}
	if p.MaxContextTokens == nil || *p.MaxContextTokens != 1048576 {
		t.Errorf("MaxContextTokens = %v, want 1048576", p.MaxContextTokens)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("bound launch copy should validate: %v", err)
	}
}

func TestApplyLaunchSelectionKeepsAnUnsetWindow(t *testing.T) {
	p := &profile.Profile{Name: "p", ProviderID: "custom", BaseURL: "https://x", ModelMode: "pick"}
	applyLaunchSelection(p, &ui.LaunchSelection{Sonnet: "m", Opus: "m", Haiku: "m", Subagent: "m"})
	if p.MaxContextTokens != nil {
		t.Errorf("MaxContextTokens = %d, want nil when the provider reported none", *p.MaxContextTokens)
	}
}
