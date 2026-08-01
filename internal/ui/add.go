package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

// AddResult holds everything collected by the add wizard.
type AddResult struct {
	Profile *profile.Profile
	APIKey  string
}

// AddProfile runs the provider-first inline add wizard.
func AddProfile() (*AddResult, error) {
	providers := provider.All()

	var providerID string
	providerOptions := make([]huh.Option[string], 0, len(providers))
	for _, p := range providers {
		providerOptions = append(providerOptions, huh.NewOption(p.DisplayName, p.ID))
	}

	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Select a provider").Options(providerOptions...).Value(&providerID),
	)).Run(); err != nil {
		return nil, err
	}

	prov, err := provider.Get(providerID)
	if err != nil {
		return nil, err
	}

	baseURL := prov.BaseURL
	var apiKey string

	var keyFields []huh.Field
	if providerID == "custom" {
		keyFields = append(keyFields,
			huh.NewInput().Title("Base URL").Description("Anthropic-compatible endpoint URL").
				Placeholder("https://api.example.com/anthropic").
				Validate(func(s string) error {
					if s == "" { return fmt.Errorf("base URL is required for custom providers") }
					return nil
				}).Value(&baseURL))
	}

	if prov.Auth.KeyURL != "" {
		fmt.Println(DimStyle.Render("  Get an API key at: " + prov.Auth.KeyURL))
	}

	keyFields = append(keyFields,
		huh.NewInput().Title("API key").EchoMode(huh.EchoModePassword).Placeholder("sk-...").
			Validate(func(s string) error {
				if s == "" { return fmt.Errorf("API key is required") }
				return nil
			}).Value(&apiKey))

	if err := huh.NewForm(huh.NewGroup(keyFields...)).Run(); err != nil {
		return nil, err
	}

	// Fetch models live
	fmt.Println(DimStyle.Render("  Fetching models from " + prov.DisplayName + "..."))
	models, err := provider.FetchModels(baseURL, prov.ModelsEndpoint(), apiKey)
	if err != nil {
		fmt.Println(Warn(fmt.Sprintf("Could not fetch models: %v", err)))
		fmt.Println(DimStyle.Render("  Falling back to manual model entry."))
		models = nil
	} else {
		fmt.Println(Success(fmt.Sprintf("Found %d models", len(models))))
	}

	// Alias — optional, auto-generated
	autoName := profile.NextAlias(providerID)
	var alias string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Profile alias (optional)").
			Description("Skip for auto-name: " + CommandStyle.Render(autoName)).
			Placeholder(autoName).
			Validate(func(s string) error {
				if s == "" { return nil }
				if err := profile.ValidName(s); err != nil { return err }
				if profile.Exists(s) { return fmt.Errorf("%q already exists", s) }
				return nil
			}).Value(&alias),
	)).Run(); err != nil {
		return nil, err
	}
	if alias == "" { alias = autoName }

	// Opus first, then rest fall back to opus.
	opusModel, sonnetModel, haikuModel, subagentModel, err := selectTiers(models)
	if err != nil {
		return nil, err
	}

	disableTraffic := prov.ID != "anthropic"
	disableBetas := prov.ID != "anthropic"
	minimalMode := false
	disableAttribution := false

	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Disable nonessential traffic").Description("Telemetry, update checks, etc.").
			Affirmative("Yes").Negative("No").Value(&disableTraffic),
		huh.NewConfirm().Title("Disable experimental betas").Description("Prevents beta headers causing errors on 3P").
			Affirmative("Yes").Negative("No").Value(&disableBetas),
		huh.NewConfirm().Title("Minimal mode (--bare)").Description("Skip hooks, CLAUDE.md, plugins, LSP").
			Affirmative("Yes").Negative("No").Value(&minimalMode),
		huh.NewConfirm().Title("Disable attribution header").Description("Strip attribution from system prompt").
			Affirmative("Yes").Negative("No").Value(&disableAttribution),
	)).Run(); err != nil {
		return nil, err
	}

	return &AddResult{
		Profile: &profile.Profile{
			Name: alias, ProviderID: providerID, BaseURL: baseURL,
			OpusModel: opusModel, SonnetModel: sonnetModel,
			HaikuModel: haikuModel, SubagentModel: subagentModel,
			DisableNonessentialTraffic: &disableTraffic,
			DisableExperimentalBetas:   &disableBetas,
			DisableAttribution:         &disableAttribution,
			MinimalMode:                minimalMode,
		},
		APIKey: apiKey,
	}, nil
}

// selectTiers — Opus first (hardest reasoning), rest fall back to it.
func selectTiers(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string, err error) {
	if len(models) == 0 {
		return selectTiersFreeText()
	}

	opts := func() []huh.Option[string] {
		out := make([]huh.Option[string], len(models))
		for i, m := range models {
			l := m.ID
			if m.OwnedBy != "" { l += "  " + DimStyle.Render("("+m.OwnedBy+")") }
			out[i] = huh.NewOption(l, m.ID)
		}
		return out
	}

	defaultModel := models[0].ID
	sameAsOpus := "(same as opus)"

	opus = defaultModel

	fields := []huh.Field{
		huh.NewSelect[string]().Title("Opus model (hard reasoning) — PRIMARY").
Options(opts()...).Value(&opus),
	}

	sonnet = defaultModel
	sOpts := opts()
	sOpts = append(sOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Sonnet model (daily driver) — optional").
Description("Defaults to same as opus").Options(sOpts...).Value(&sonnet))

	haiku = defaultModel
	hOpts := opts()
	hOpts = append(hOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Haiku model (fast/cheap) — optional").
Description("Defaults to same as opus").Options(hOpts...).Value(&haiku))

	subagent = defaultModel
	subOpts := opts()
	subOpts = append(subOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Subagent model — optional").
Description("Defaults to same as opus").Options(subOpts...).Value(&subagent))

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", "", "", "", err
	}
	return opus, sonnet, haiku, subagent, nil
}

func selectTiersFreeText() (opus, sonnet, haiku, subagent string, err error) {
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Opus model ID (hard reasoning) — PRIMARY").Placeholder("model-id").
			Validate(func(s string) error { if s == "" { return fmt.Errorf("required") }; return nil }).Value(&opus),
		huh.NewInput().Title("Sonnet model ID — optional").Placeholder("same as opus").Value(&sonnet),
		huh.NewInput().Title("Haiku model ID — optional").Placeholder("same as opus").Value(&haiku),
		huh.NewInput().Title("Subagent model ID — optional").Placeholder("same as opus").Value(&subagent),
	)).Run(); err != nil {
		return "", "", "", "", err
	}
	return opus, sonnet, haiku, subagent, nil
}
