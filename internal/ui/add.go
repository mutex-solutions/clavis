package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

type AddResult struct {
	Profile *profile.Profile
	APIKey  string
}

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

	fmt.Println(DimStyle.Render("  Fetching models from " + prov.DisplayName + "..."))
	models, err := provider.FetchModels(baseURL, prov.ModelsEndpoint(), apiKey)
	if err != nil {
		fmt.Println(Warn(fmt.Sprintf("Could not fetch models: %v", err)))
		fmt.Println(DimStyle.Render("  Falling back to manual model entry."))
		models = nil
	} else {
		fmt.Println(Success(fmt.Sprintf("Found %d models", len(models))))
	}

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

	opusModel, sonnetModel, haikuModel, subagentModel := selectModels(models)

	// Advanced options
	var showAdvanced bool
	var effort string
	var noThinking bool
	var compactPct int

	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Advanced configuration?").
			Description("Fine-tune effort, thinking, and compaction. Defaults work for most providers.").
			Affirmative("Yes, customize").Negative("No, use defaults").Value(&showAdvanced),
	)).Run(); err != nil {
		return nil, err
	}

	if showAdvanced {
		compactPct, effort, noThinking = selectAdvancedOptions()
	}

	is3P := prov.ID != "anthropic"
	p := &profile.Profile{
		Name: alias, ProviderID: providerID, BaseURL: baseURL,
		OpusModel: opusModel, SonnetModel: sonnetModel,
		HaikuModel: haikuModel, SubagentModel: subagentModel,
		DisableNonessentialTraffic: &is3P,
		DisableExperimentalBetas:   &is3P,
		DisableAttribution:         &is3P,
	}

	if effort != "" { p.EffortLevel = effort }
	if noThinking { p.DisableThinking = true }
	if compactPct > 0 { p.AutoCompactPct = &compactPct }

	return &AddResult{Profile: p, APIKey: apiKey}, nil
}

func selectModels(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string) {
	if len(models) == 0 {
		return selectTiersFreeText()
	}

	// Pre-filter for large model lists (> 50) to avoid huh rendering bugs.
	filtered := models
	if len(models) > 50 {
		var search string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Search models").
				Description(fmt.Sprintf("%d models available — type to filter", len(models))).
				Placeholder("type part of model name...").
				Value(&search),
		)).Run(); err != nil {
			return "", "", "", ""
		}
		if search != "" {
			s := strings.ToLower(search)
			for _, m := range models {
				if strings.Contains(strings.ToLower(m.ID), s) {
					filtered = append(filtered, m)
				}
			}
		}
		if len(filtered) == 0 {
			filtered = models // no matches, show all
		}
	}

	var simple bool
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Simple model setup?").
			Description("One model for all tiers. Choose 'No' to pick per-tier.").
			Affirmative("Yes, one model").Negative("No, per-tier").Value(&simple),
	)).Run(); err != nil {
		return "", "", "", ""
	}

	if simple {
		m := selectOneModel(filtered)
		return m, m, m, m
	}
	return selectTiersDetailed(filtered)
}

func selectOneModel(models []provider.ModelInfo) string {
	opts := make([]huh.Option[string], len(models))
	for i, m := range models {
		l := m.ID
		if m.OwnedBy != "" { l += "  " + DimStyle.Render("("+m.OwnedBy+")") }
		opts[i] = huh.NewOption(l, m.ID)
	}
	m := models[0].ID
	huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Select model").
			Description(DimStyle.Render("Press / to search, ↑↓ to navigate")).
			Options(opts...).Value(&m),
	)).Run()
	return m
}

func selectTiersDetailed(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string) {
	opts := func() []huh.Option[string] {
		out := make([]huh.Option[string], len(models))
		for i, m := range models {
			l := m.ID
			if m.OwnedBy != "" { l += "  " + DimStyle.Render("("+m.OwnedBy+")") }
			out[i] = huh.NewOption(l, m.ID)
		}
		return out
	}

	filterHint := DimStyle.Render("Press / to search, ↑↓ to navigate, enter to select")
	defaultModel := models[0].ID
	sameAsOpus := "(same as opus)"

	opus = defaultModel
	fields := []huh.Field{
		huh.NewSelect[string]().Title("Opus model (hard reasoning)").
			Description(filterHint).Options(opts()...).Value(&opus),
	}

	sonnet = defaultModel
	sOpts := opts()
	sOpts = append(sOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Sonnet model (daily driver)").
			Description("Defaults to same as opus. " + filterHint).Options(sOpts...).Value(&sonnet))

	haiku = defaultModel
	hOpts := opts()
	hOpts = append(hOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Haiku model (fast/cheap)").
			Description("Defaults to same as opus").Options(hOpts...).Value(&haiku))

	subagent = defaultModel
	subOpts := opts()
	subOpts = append(subOpts, huh.NewOption(sameAsOpus, ""))
	fields = append(fields,
		huh.NewSelect[string]().Title("Subagent model").
			Description("Defaults to same as opus").Options(subOpts...).Value(&subagent))

	huh.NewForm(huh.NewGroup(fields...)).Run()
	return opus, sonnet, haiku, subagent
}

func selectAdvancedOptions() (compactPct int, effort string, noThinking bool) {
	effort = "auto"

	var compactStr string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Effort level").
			Description("How hard Claude tries. Higher = slower but better reasoning.").
			Options(
				huh.NewOption("auto (model default)", "auto"),
				huh.NewOption("low — fast, less thorough", "low"),
				huh.NewOption("medium", "medium"),
				huh.NewOption("high — thorough reasoning", "high"),
				huh.NewOption("xhigh — very thorough", "xhigh"),
				huh.NewOption("max — maximum reasoning depth", "max"),
			).Value(&effort),

		huh.NewConfirm().Title("Disable thinking tokens?").
			Description("Omit thinking/reasoning tokens. Needed for providers that don't support them.").
			Affirmative("Yes, disable").Negative("No, keep enabled").Value(&noThinking),

		huh.NewInput().Title("Auto-compact threshold (%)").
			Description("When to compact context. Lower = compact sooner. Default: 80%. Leave empty to skip.").
			Placeholder("80").
			Validate(func(s string) error {
				if s == "" { return nil }
				v, err := strconv.Atoi(s)
				if err != nil || v < 1 || v > 100 {
					return fmt.Errorf("must be 1–100")
				}
				return nil
			}).Value(&compactStr),
	)).Run(); err != nil {
		return 0, "auto", false
	}

	if compactStr != "" {
		compactPct, _ = strconv.Atoi(compactStr)
	}
	return
}

func selectTiersFreeText() (opus, sonnet, haiku, subagent string) {
	huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Opus model ID — PRIMARY").Placeholder("model-id").
			Validate(func(s string) error { if s == "" { return fmt.Errorf("required") }; return nil }).Value(&opus),
		huh.NewInput().Title("Sonnet model ID — optional").Placeholder("same as opus").Value(&sonnet),
		huh.NewInput().Title("Haiku model ID — optional").Placeholder("same as opus").Value(&haiku),
		huh.NewInput().Title("Subagent model ID — optional").Placeholder("same as opus").Value(&subagent),
	)).Run()
	return opus, sonnet, haiku, subagent
}
