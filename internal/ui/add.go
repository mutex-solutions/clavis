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
// Flow: provider → key → fetch models → alias (optional) → tiers → options.
func AddProfile() (*AddResult, error) {
	providers := provider.All()

	// Step 1: Provider selection (FIRST)
	var providerID string
	providerOptions := make([]huh.Option[string], 0, len(providers))
	for _, p := range providers {
		providerOptions = append(providerOptions, huh.NewOption(p.DisplayName, p.ID))
	}

	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select a provider").
			Options(providerOptions...).
			Value(&providerID),
	)).Run(); err != nil {
		return nil, err
	}

	prov, err := provider.Get(providerID)
	if err != nil {
		return nil, err
	}

	// Step 2: Base URL (custom only) + API Key
	baseURL := prov.BaseURL
	var apiKey string

	var keyFields []huh.Field
	if providerID == "custom" {
		keyFields = append(keyFields,
			huh.NewInput().Title("Base URL").
				Description("Anthropic-compatible endpoint URL").
				Placeholder("https://api.example.com/anthropic").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("base URL is required for custom providers")
					}
					return nil
				}).Value(&baseURL))
	}

	if prov.Auth.KeyURL != "" {
		fmt.Println(DimStyle.Render("  Get an API key at: " + prov.Auth.KeyURL))
	}

	keyFields = append(keyFields,
		huh.NewInput().Title("API key").
			EchoMode(huh.EchoModePassword).
			Placeholder("sk-...").
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("API key is required")
				}
				return nil
			}).Value(&apiKey))

	if err := huh.NewForm(huh.NewGroup(keyFields...)).Run(); err != nil {
		return nil, err
	}

	// Step 3: Fetch models live from the provider
	fmt.Println(DimStyle.Render("  Fetching models from " + prov.DisplayName + "..."))
	models, err := provider.FetchModels(baseURL, apiKey)
	if err != nil {
		fmt.Println(Warn(fmt.Sprintf("Could not fetch models: %v", err)))
		fmt.Println(DimStyle.Render("  Falling back to manual model entry."))
		models = nil
	} else {
		fmt.Println(Success(fmt.Sprintf("Found %d models", len(models))))
	}

	// Step 4: Alias — OPTIONAL, auto-generated if skipped
	autoName := profile.NextAlias(providerID)
	var alias string

	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Profile alias (optional)").
			Description("Skip for auto-name: " + CommandStyle.Render(autoName)).
			Placeholder(autoName).
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				if err := profile.ValidName(s); err != nil {
					return err
				}
				if profile.Exists(s) {
					return fmt.Errorf("%q already exists", s)
				}
				return nil
			}).Value(&alias),
	)).Run(); err != nil {
		return nil, err
	}
	if alias == "" {
		alias = autoName
	}

	// Step 5: Model tier selection
	sonnetModel, opusModel, haikuModel, subagentModel, err := selectTiers(models)
	if err != nil {
		return nil, err
	}

	// Step 6: Options
	disableTraffic := prov.ID != "anthropic"
	disableBetas := prov.ID != "anthropic"
	minimalMode := false
	disableAttribution := false

	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Disable nonessential traffic").
			Description("Telemetry, update checks, etc.").
			Affirmative("Yes").Negative("No").Value(&disableTraffic),
		huh.NewConfirm().Title("Disable experimental betas").
			Description("Prevents beta headers causing errors on 3P").
			Affirmative("Yes").Negative("No").Value(&disableBetas),
		huh.NewConfirm().Title("Minimal mode (--bare)").
			Description("Skip hooks, CLAUDE.md, plugins, LSP").
			Affirmative("Yes").Negative("No").Value(&minimalMode),
		huh.NewConfirm().Title("Disable attribution header").
			Description("Strip attribution from system prompt").
			Affirmative("Yes").Negative("No").Value(&disableAttribution),
	)).Run(); err != nil {
		return nil, err
	}

	return &AddResult{
		Profile: &profile.Profile{
			Name:                       alias,
			ProviderID:                 providerID,
			BaseURL:                    baseURL,
			SonnetModel:                sonnetModel,
			OpusModel:                  opusModel,
			HaikuModel:                 haikuModel,
			SubagentModel:              subagentModel,
			DisableNonessentialTraffic: &disableTraffic,
			DisableExperimentalBetas:   &disableBetas,
			DisableAttribution:         &disableAttribution,
			MinimalMode:                minimalMode,
		},
		APIKey: apiKey,
	}, nil
}

// selectTiers runs dynamic model tier selection from live-fetched models.
func selectTiers(models []provider.ModelInfo) (sonnet, opus, haiku, subagent string, err error) {
	if len(models) == 0 {
		return selectTiersFreeText()
	}

	modelOptions := func() []huh.Option[string] {
		opts := make([]huh.Option[string], len(models))
		for i, m := range models {
			label := m.ID
			if m.OwnedBy != "" {
				label += "  " + DimStyle.Render("("+m.OwnedBy+")")
			}
			opts[i] = huh.NewOption(label, m.ID)
		}
		return opts
	}

	sameAsSonnet := "(same as sonnet)"
	defaultModel := models[0].ID

	sonnet = defaultModel

	fields := []huh.Field{
		huh.NewSelect[string]().Title("Sonnet model (daily driver) — REQUIRED").
			Options(modelOptions()...).Value(&sonnet),
	}

	opus = defaultModel
	opusOpts := modelOptions()
	opusOpts = append(opusOpts, huh.NewOption(sameAsSonnet, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Opus model (hard reasoning) — optional").
			Options(opusOpts...).Value(&opus))

	haiku = defaultModel
	haikuOpts := modelOptions()
	haikuOpts = append(haikuOpts, huh.NewOption(sameAsSonnet, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Haiku model (fast/cheap) — optional").
			Options(haikuOpts...).Value(&haiku))

	subagent = defaultModel
	subOpts := modelOptions()
	subOpts = append(subOpts, huh.NewOption(sameAsSonnet, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Subagent model — optional").
			Options(subOpts...).Value(&subagent))

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return "", "", "", "", err
	}
	return sonnet, opus, haiku, subagent, nil
}

func selectTiersFreeText() (sonnet, opus, haiku, subagent string, err error) {
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Sonnet model ID (daily driver) — REQUIRED").
			Placeholder("model-id").Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("required")
			}
			return nil
		}).Value(&sonnet),
		huh.NewInput().Title("Opus model ID — optional").
			Placeholder("same as sonnet").Value(&opus),
		huh.NewInput().Title("Haiku model ID — optional").
			Placeholder("same as sonnet").Value(&haiku),
		huh.NewInput().Title("Subagent model ID — optional").
			Placeholder("same as sonnet").Value(&subagent),
	)).Run(); err != nil {
		return "", "", "", "", err
	}
	return sonnet, opus, haiku, subagent, nil
}
