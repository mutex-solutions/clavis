package main

import (
	"testing"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
)

func window(v int) *int { return &v }

func TestApplyPickedModelMovesTiersThatFollowedThePrimary(t *testing.T) {
	p := &profile.Profile{
		SonnetModel:   "old",
		OpusModel:     "old",
		HaikuModel:    "old",
		SubagentModel: "old",
	}
	models := []provider.ModelInfo{{ID: "new", MaxInputTokens: window(1048576)}}

	applyPickedModel(p, models, "new")

	for name, got := range map[string]string{
		"sonnet":   p.SonnetModel,
		"opus":     p.OpusModel,
		"haiku":    p.HaikuModel,
		"subagent": p.SubagentModel,
	} {
		if got != "new" {
			t.Errorf("%s model = %q, want new", name, got)
		}
	}
	if p.MaxContextTokens == nil || *p.MaxContextTokens != 1048576 {
		t.Errorf("MaxContextTokens = %v, want 1048576", p.MaxContextTokens)
	}
}

func TestApplyPickedModelKeepsTiersWithTheirOwnModel(t *testing.T) {
	p := &profile.Profile{
		SonnetModel:   "old",
		OpusModel:     "strong",
		HaikuModel:    "fast",
		SubagentModel: "old",
	}
	applyPickedModel(p, []provider.ModelInfo{{ID: "new"}}, "new")

	if p.OpusModel != "strong" {
		t.Errorf("opus = %q, want strong", p.OpusModel)
	}
	if p.HaikuModel != "fast" {
		t.Errorf("haiku = %q, want fast", p.HaikuModel)
	}
	if p.SonnetModel != "new" || p.SubagentModel != "new" {
		t.Errorf("sonnet/subagent = %q/%q, want new/new", p.SonnetModel, p.SubagentModel)
	}
}

func TestApplyPickedModelClearsAWindowItCannotVerify(t *testing.T) {
	p := &profile.Profile{SonnetModel: "old", MaxContextTokens: window(200000)}
	applyPickedModel(p, []provider.ModelInfo{{ID: "new"}}, "new")
	if p.MaxContextTokens != nil {
		t.Errorf("MaxContextTokens = %d, want nil when the new model reports no window", *p.MaxContextTokens)
	}
}
