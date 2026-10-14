package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/mutex-solutions/clavis/internal/profile"
)

// The non-profile rows in the selector. They can't collide with a real entry
// because profile names may not contain underscores.
const (
	selectorAdd  = "__add__"
	selectorEdit = "__edit__"
)

// SelectorResult is what the profile selector returns.
type SelectorResult struct {
	Profile *profile.Profile
	Action  string // "launch", "add", "edit", "quit"
}

// SelectProfile shows an inline profile picker.
func SelectProfile(profiles []*profile.Profile, lastUsed string) (*SelectorResult, error) {
	if len(profiles) == 0 {
		return &SelectorResult{Action: "add"}, nil
	}

	options := make([]huh.Option[string], 0, len(profiles)+2)
	for _, p := range profiles {
		label := fmt.Sprintf("%-14s %s (%s)", p.Name, p.ProviderID, p.SonnetModel)
		// The context window is the one setting that silently changes how much
		// work fits in a session, so show it wherever profiles are compared.
		if p.MaxContextTokens != nil {
			label += " " + DimStyle.Render(FormatTokens(*p.MaxContextTokens))
		}
		options = append(options, huh.NewOption(label, p.Name))
	}
	options = append(options,
		huh.NewOption("── add new profile ──", selectorAdd),
		huh.NewOption("── edit a profile ──", selectorEdit),
	)

	// Pre-select last used
	selected := ""
	if lastUsed != "" {
		for _, p := range profiles {
			if p.Name == lastUsed {
				selected = p.Name
				break
			}
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a profile").
				Description(DimStyle.Render("esc = quit")).
				Options(options...).
				Value(&selected),
		),
	)

	if err := runForm(form); err != nil {
		return &SelectorResult{Action: "quit"}, nil
	}

	if selected == selectorAdd {
		return &SelectorResult{Action: "add"}, nil
	}
	if selected == selectorEdit {
		return &SelectorResult{Action: "edit"}, nil
	}

	for _, p := range profiles {
		if p.Name == selected {
			return &SelectorResult{Profile: p, Action: "launch"}, nil
		}
	}

	return &SelectorResult{Action: "quit"}, nil
}

// ConfirmDelete asks for delete confirmation.
func ConfirmDelete(name string) bool {
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete profile %q?", name)).
				Description("This removes the profile and its API key.").
				Affirmative("Delete").
				Negative("Cancel").
				Value(&confirm),
		),
	)
	if err := runForm(form); err != nil {
		return false
	}
	return confirm
}
