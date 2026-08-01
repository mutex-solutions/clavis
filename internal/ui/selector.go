package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/dinhlongviolin1/clavis/internal/profile"
)

// SelectorResult is what the profile selector returns.
type SelectorResult struct {
	Profile *profile.Profile
	Action  string // "launch", "add", "delete", "quit"
}

// SelectProfile shows an inline profile picker.
func SelectProfile(profiles []*profile.Profile, lastUsed string) (*SelectorResult, error) {
	if len(profiles) == 0 {
		return &SelectorResult{Action: "add"}, nil
	}

	options := make([]huh.Option[string], 0, len(profiles)+1)
	for _, p := range profiles {
		label := fmt.Sprintf("%-14s %s (%s)", p.Name, p.ProviderID, p.SonnetModel)
		options = append(options, huh.NewOption(label, p.Name))
	}
	options = append(options, huh.NewOption("── add new profile ──", "__add__"))

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

	if selected == "__add__" {
		return &SelectorResult{Action: "add"}, nil
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
