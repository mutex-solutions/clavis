package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
)

// EditResult is what the profile editor hands back on save. The caller writes
// it to disk — the editor itself touches nothing.
type EditResult struct {
	// Profile is the edited copy.
	Profile *profile.Profile
	// APIKey is set only when the user typed a new key: `profile.Save` skips
	// the .env write on an empty one, so the stored key survives untouched.
	APIKey string
}

// The settings menu rows. Constants rather than the labels so the label text
// can change without silently breaking the switch.
const (
	editContext  = "context"
	editModels   = "models"
	editEffort   = "effort"
	editThinking = "thinking"
	editCompact  = "compact"
	editIsolate  = "isolation"
	editBare     = "bare"
	editKey      = "key"
	editDone     = "done"
)

// EditProfile walks an existing profile's settings, one screen at a time.
//
// It mutates a copy and never writes to disk, so Esc anywhere (or Ctrl+C, which
// exits) discards the session's changes. `apiKey` is the key already stored; it
// is used only to fetch the provider's model list, and is returned only if the
// user replaces it.
func EditProfile(p *profile.Profile, apiKey string) (*EditResult, error) {
	edited := *p
	newKey := ""

	var (
		models  []provider.ModelInfo
		fetched bool
	)
	// Fetched at most once, and the failure is cached too: the call needs the
	// network, and an unreachable provider must not block changing the context
	// window — nor re-hit the network every time the menu comes back around.
	modelsFor := func() []provider.ModelInfo {
		if !fetched {
			models = fetchModelsForEdit(&edited, apiKey)
			fetched = true
		}
		return models
	}

	for {
		clearScreen()
		printEditSummary(&edited)

		choice := editContext
		if err := runForm(huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Edit "+edited.Name).
				Description(DimStyle.Render("esc = cancel — changes are discarded until you pick Done")).
				Options(
					huh.NewOption("Context window", editContext),
					huh.NewOption("Models (per tier)", editModels),
					huh.NewOption("Effort level", editEffort),
					huh.NewOption("Thinking tokens", editThinking),
					huh.NewOption("Auto-compact threshold", editCompact),
					huh.NewOption("Config isolation", editIsolate),
					huh.NewOption("Minimal mode (--bare)", editBare),
					huh.NewOption("API key", editKey),
					huh.NewOption("Done — save", editDone),
				).Value(&choice),
		))); err != nil {
			return nil, errCancel
		}

		switch choice {
		case editDone:
			return &EditResult{Profile: &edited, APIKey: newKey}, nil

		case editContext:
			detected := detectedContextOptions(modelsFor(),
				edited.SonnetModel, edited.OpusModel, edited.HaikuModel, edited.SubagentModel)
			ctx, err := wizardContextWindow(detected, edited.MaxContextTokens)
			if err != nil {
				continue // esc = back to the menu, keeping everything so far
			}
			edited.MaxContextTokens = ctx

		case editModels:
			o, s, h, sub, err := wizardModelSelect(modelsFor(), &edited)
			if err != nil {
				continue
			}
			// Free-text entry (the fallback when the provider can't be reached)
			// can't pre-fill, and an empty optional tier there means "same as
			// opus" — which cannot be what an edit to one tier intends. So an
			// empty answer keeps the current model instead.
			if o == "" {
				o = edited.OpusModel
			}
			if s == "" {
				s = edited.SonnetModel
			}
			if h == "" {
				h = edited.HaikuModel
			}
			if sub == "" {
				sub = edited.SubagentModel
			}
			edited.OpusModel, edited.SonnetModel = o, s
			edited.HaikuModel, edited.SubagentModel = h, sub

		case editEffort:
			e, err := wizardEffort(edited.EffortLevel)
			if err != nil {
				continue
			}
			edited.EffortLevel = e

		case editThinking:
			nt, err := wizardThinking(edited.DisableThinking)
			if err != nil {
				continue
			}
			edited.DisableThinking = nt

		case editCompact:
			cp, err := wizardCompactPct(edited.AutoCompactPct)
			if err != nil {
				continue
			}
			edited.AutoCompactPct = cp

		case editIsolate:
			iso, err := wizardIsolation(edited.IsolatedConfig)
			if err != nil {
				continue
			}
			edited.IsolatedConfig = iso

		case editBare:
			m, err := wizardMinimal(edited.MinimalMode)
			if err != nil {
				continue
			}
			edited.MinimalMode = m

		case editKey:
			k, err := wizardAPIKey(apiKey != "")
			if err != nil {
				continue
			}
			if k != "" {
				newKey = k
			}
		}
	}
}

// printEditSummary prints the profile's current state above the menu, so the
// effect of each change is visible before it is saved.
func printEditSummary(p *profile.Profile) {
	fmt.Println(TitleStyle.Render("Edit profile"))
	fmt.Println()

	row := func(label, val string) {
		fmt.Printf("  %s %s\n", DimStyle.Render(fmt.Sprintf("%-14s", label)), val)
	}
	// Tiers shown as the fallback they resolve to, since an empty tier is a
	// real state ("use the main model") rather than a missing value.
	tier := func(name, model, fallback string) {
		if model == "" {
			row(name, DimStyle.Render("same as main ("+fallback+")"))
			return
		}
		row(name, model)
	}
	row("Name", p.Name)
	row("Provider", p.ProviderID)
	row("Base URL", p.BaseURL)
	row("Main model", p.SonnetModel)
	tier("Reasoning", p.OpusModel, p.SonnetModel)
	tier("Fast", p.HaikuModel, p.SonnetModel)
	tier("Subagent", p.SubagentModel, p.SonnetModel)

	if p.MaxContextTokens != nil {
		row("Context", FormatTokens(*p.MaxContextTokens))
	} else {
		row("Context", DimStyle.Render("guessed by Claude Code (200K for unrecognised IDs)"))
	}
	if p.AutoCompactPct != nil {
		row("Compacts at", fmt.Sprintf("%d%% of the window", *p.AutoCompactPct))
	}
	if p.AutoCompactWindow != nil {
		row("Compacts at", FormatTokens(*p.AutoCompactWindow)+" tokens")
	}
	if p.EffortLevel != "" {
		row("Effort", p.EffortLevel)
	}
	row("Thinking", onOff(!p.DisableThinking, "enabled", "disabled"))
	row("Config", onOff(p.IsolatedConfig, "isolated", "shared with ~/.claude"))
	row("Mode", onOff(p.MinimalMode, "minimal (--bare)", "full"))
	fmt.Println()
}

// onOff renders a boolean as the word for its current state.
func onOff(v bool, on, off string) string {
	if v {
		return on
	}
	return off
}

// fetchModelsForEdit lists the provider's models using the key already on disk,
// so the editor can offer the real model list and any context windows the
// provider reports. Best-effort by design: a failure returns nil and the
// callers fall back to presets and free-text entry.
func fetchModelsForEdit(p *profile.Profile, apiKey string) []provider.ModelInfo {
	if apiKey == "" {
		return nil
	}
	prov, provErr := provider.Get(p.ProviderID)
	if provErr == nil && len(prov.Models) > 0 {
		// The endpoint has no listing; the declared catalogue is the only
		// honest answer and needs no request.
		return prov.Models
	}
	endpoint := p.BaseURL // provider.ModelsEndpoint falls back to this
	if provErr == nil {
		endpoint = prov.ModelsEndpoint()
	}
	res, err := runWithSpinner("Checking the provider…", func() (any, error) {
		return provider.FetchModels(p.BaseURL, endpoint, apiKey)
	})
	if err != nil || res == nil {
		return nil
	}
	models, _ := res.([]provider.ModelInfo)
	return models
}

// SelectProfileForEdit asks which profile to edit, when none was named.
// Returns errCancel if the user backs out.
func SelectProfileForEdit(profiles []*profile.Profile) (string, error) {
	options := make([]huh.Option[string], 0, len(profiles))
	for _, p := range profiles {
		label := fmt.Sprintf("%-14s %s", p.Name, p.SonnetModel)
		if p.MaxContextTokens != nil {
			label += DimStyle.Render("  " + FormatTokens(*p.MaxContextTokens))
		}
		options = append(options, huh.NewOption(label, p.Name))
	}

	// Seed the bound value so the cursor starts on the first profile.
	chosen := ""
	if len(profiles) > 0 {
		chosen = profiles[0].Name
	}

	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Edit which profile?").
			Description(DimStyle.Render("esc = cancel")).
			Options(options...).Value(&chosen),
	))); err != nil {
		return "", errCancel
	}
	return chosen, nil
}
