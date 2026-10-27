package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mutex-solutions/clavis/internal/provider"
)

// LaunchSelection is what a pick-mode profile resolves to for one launch: a
// model per tier, plus an optional context-window override. It is applied to a
// session copy of the profile and never saved.
type LaunchSelection struct {
	Opus, Sonnet, Haiku, Subagent string
	MaxContext                    *int
}

// PickLaunchModels walks a pick-mode profile through its choices: the main
// model, whether every tier uses it or each tier gets its own, and an optional
// context-window override (the detected window is the default).
//
// Esc backs out to the caller, which cancels the launch.
func PickLaunchModels(models []provider.ModelInfo) (*LaunchSelection, error) {
	clearScreen()
	sonnet, err := RunModelSelect("Select the main model", models, 12, "", nil)
	if err != nil || sonnet == "" {
		return nil, errBack
	}

	const (
		optSame = "same"
		optTier = "tier"
	)
	choice := optSame
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Model tiers").
			Description(DimStyle.Render("esc = cancel launch")).
			Options(
				huh.NewOption("Use this model for every tier", optSame),
				huh.NewOption("Choose a model for each tier", optTier),
			).Value(&choice),
	))); err != nil {
		return nil, errBack
	}

	sel := &LaunchSelection{Sonnet: sonnet, Opus: sonnet, Haiku: sonnet, Subagent: sonnet}
	if choice == optTier {
		if sel.Opus, err = tierModel("Opus tier (hard reasoning)", models, sonnet); err != nil {
			return nil, err
		}
		if sel.Haiku, err = tierModel("Haiku tier (fast/cheap)", models, sonnet); err != nil {
			return nil, err
		}
		if sel.Subagent, err = tierModel("Subagent tier", models, sonnet); err != nil {
			return nil, err
		}
	}

	detected := provider.ContextWindowFor(models, sonnet)
	var input string
	placeholder := ""
	hint := "enter = use the window the provider reports"
	if detected > 0 {
		placeholder = strconv.Itoa(detected)
		hint += " (" + FormatTokens(detected) + ")"
	}
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Context window (tokens)").
			Description(DimStyle.Render(hint)).
			Placeholder(placeholder).
			Validate(func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return nil
				}
				n, err := strconv.Atoi(s)
				if err != nil || n <= 0 {
					return fmt.Errorf("enter a positive token count, or leave it empty")
				}
				return nil
			}).Value(&input),
	))); err != nil {
		return nil, errBack
	}

	if s := strings.TrimSpace(input); s != "" {
		n, _ := strconv.Atoi(s)
		sel.MaxContext = &n
	} else if detected > 0 {
		sel.MaxContext = &detected
	}
	return sel, nil
}

// tierModel asks for one tier's model, with "same as main" as the default.
func tierModel(title string, models []provider.ModelInfo, main string) (string, error) {
	chosen, err := RunModelSelect(title, models, 12, "Same as main", []string{main})
	if err != nil {
		return "", errBack
	}
	if chosen == "" {
		return main, nil
	}
	return chosen, nil
}
