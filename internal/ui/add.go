package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

// errBack signals that the user wants to go to the previous step.
var errBack = errors.New("go back")

type AddResult struct {
	Profile *profile.Profile
	APIKey  string
}

func AddProfile() (*AddResult, error) {
	var (
		providerID                                                        string
		baseURL                                                           string
		apiKey                                                            string
		models                                                            []provider.ModelInfo
		alias                                                             string
		opusModel, sonnetModel, haikuModel, subagentModel                 string
		effort                                                            string
		noThinking                                                        bool
		compactPct                                                        int
	)

	// Step 1: Provider
	step := 0
	for {
		switch step {
		case 0:
			p, url, key, err := wizardProvider()
			if err != nil {
				if errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted) { continue } // first step, no back
				return nil, err
			}
			providerID = p
			baseURL = url
			apiKey = key
			step = 1
		case 1:
			m, err := wizardModels(baseURL, apiKey)
			if err != nil {
				if errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted) { step = 0; continue }
				return nil, err
			}
			models = m
			step = 2
		case 2:
			a, err := wizardAlias(providerID)
			if err != nil {
				if errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted) { step = 1; continue }
				return nil, err
			}
			alias = a
			step = 3
		case 3:
			o, s, h, sub, err := wizardModelSelect(models)
			if err != nil {
				if errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted) { step = 2; continue }
				return nil, err
			}
			opusModel, sonnetModel, haikuModel, subagentModel = o, s, h, sub
			step = 4
		case 4:
			e, nt, cp, err := wizardAdvanced()
			if err != nil {
				if errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted) { step = 3; continue }
				return nil, err
			}
			effort = e
			noThinking = nt
			compactPct = cp
			// Build profile and return.
			is3P := providerID != "anthropic"
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
	}
}

func wizardProvider() (providerID, baseURL, apiKey string, err error) {
	providers := provider.All()
	providerOptions := make([]huh.Option[string], 0, len(providers))
	for _, p := range providers {
		providerOptions = append(providerOptions, huh.NewOption(p.DisplayName, p.ID))
	}

	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Select a provider").
			Description(DimStyle.Render("esc = cancel")).Options(providerOptions...).Value(&providerID),
	)).Run(); err != nil {
		return "", "", "", err
	}

	prov, err := provider.Get(providerID)
	if err != nil {
		return "", "", "", err
	}
	baseURL = prov.BaseURL

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
			Description(DimStyle.Render("esc = go back")).
			Validate(func(s string) error {
				if s == "" { return fmt.Errorf("API key is required") }
				return nil
			}).Value(&apiKey))

	if err := huh.NewForm(huh.NewGroup(keyFields...)).Run(); err != nil {
		return "", "", "", errBack
	}
	return providerID, baseURL, apiKey, nil
}

func wizardModels(baseURL, apiKey string) ([]provider.ModelInfo, error) {
	prov, _ := provider.Get("") // ignore error — we just need display name fallback
	_ = prov

	fmt.Println(DimStyle.Render("  Fetching models..."))
	models, err := provider.FetchModels(baseURL, "", apiKey)
	if err != nil {
		fmt.Println(Warn(fmt.Sprintf("Could not fetch models: %v", err)))
		fmt.Println(DimStyle.Render("  Falling back to manual model entry."))
		return nil, nil
	}
	fmt.Println(Success(fmt.Sprintf("Found %d models — esc to go back, enter to continue", len(models))))
	return models, nil
}

func wizardAlias(providerID string) (string, error) {
	autoName := profile.NextAlias(providerID)
	var alias string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Profile alias (optional)").
			Description("Skip for auto-name: " + CommandStyle.Render(autoName) + "  " + DimStyle.Render("esc = go back")).
			Placeholder(autoName).
			Validate(func(s string) error {
				if s == "" { return nil }
				if err := profile.ValidName(s); err != nil { return err }
				if profile.Exists(s) { return fmt.Errorf("%q already exists", s) }
				return nil
			}).Value(&alias),
	)).Run(); err != nil {
		return "", errBack
	}
	if alias == "" { alias = autoName }
	return alias, nil
}

func wizardModelSelect(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string, err error) {
	if len(models) == 0 {
		o, s, h, sub, e := selectTiersFreeText(); return o, s, h, sub, e
	}

	filtered := models
	if len(models) > 50 {
		var search string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Filter models").
				Description(fmt.Sprintf("%d models — type to filter, esc = go back", len(models))).
				Placeholder("type part of model name...").Value(&search),
		)).Run(); err != nil {
			return "", "", "", "", errBack
		}
		if search != "" {
			filtered = nil
			s := strings.ToLower(search)
			for _, m := range models {
				if strings.Contains(strings.ToLower(m.ID), s) {
					filtered = append(filtered, m)
				}
			}
		}
		if len(filtered) == 0 { filtered = models }
	}

	var simple bool
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Simple model setup?").
			Description("One model for all tiers. esc = go back.").
			Affirmative("Yes, one model").Negative("No, per-tier").Value(&simple),
	)).Run(); err != nil {
		return "", "", "", "", errBack
	}

	if simple {
		m := selectOneModel(filtered)
		return m, m, m, m, nil
	}
	o, s, h, sub, err := selectTiersDetailed(filtered); return o, s, h, sub, err
}

func wizardAdvanced() (effort string, noThinking bool, compactPct int, err error) {
	var show bool
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Advanced configuration?").
			Description("Effort, thinking, compaction. esc = skip and use defaults.").
			Affirmative("Yes, customize").Negative("No, use defaults").Value(&show),
	)).Run(); err != nil {
		return "", false, 0, nil // esc = skip advanced
	}
	if !show {
		return "", false, 0, nil
	}

	effort = "auto"
	var compactStr string
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Effort level").
			Description(DimStyle.Render("esc = go back")).
			Options(
				huh.NewOption("auto (model default)", "auto"),
				huh.NewOption("low — fast, less thorough", "low"),
				huh.NewOption("medium", "medium"),
				huh.NewOption("high — thorough reasoning", "high"),
				huh.NewOption("xhigh — very thorough", "xhigh"),
				huh.NewOption("max — maximum reasoning depth", "max"),
			).Value(&effort),
		huh.NewConfirm().Title("Disable thinking tokens?").
			Description("Omit thinking tokens. Needed for providers that don't support them.").
			Affirmative("Yes, disable").Negative("No").Value(&noThinking),
		huh.NewInput().Title("Auto-compact threshold (%)").
			Description("Lower = compact sooner. Default: 80%. Empty = skip.").
			Placeholder("80").
			Validate(func(s string) error {
				if s == "" { return nil }
				v, err := strconv.Atoi(s)
				if err != nil || v < 1 || v > 100 { return fmt.Errorf("must be 1–100") }
				return nil
			}).Value(&compactStr),
	)).Run(); err != nil {
		return "", false, 0, errBack
	}
	if compactStr != "" { compactPct, _ = strconv.Atoi(compactStr) }
	return effort, noThinking, compactPct, nil
}

func selectOneModel(models []provider.ModelInfo) string {
	opts := make([]huh.Option[string], len(models))
	for i, m := range models {
		l := m.ID
		if m.OwnedBy != "" { l += "  " + DimStyle.Render("("+m.OwnedBy+")") }
		opts[i] = huh.NewOption(l, m.ID)
	}
	m := models[0].ID
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Height(5).Title("Select model").
			Description(DimStyle.Render("/ = search, ↑↓ = navigate, esc = go back")).
			Options(opts...).Value(&m),
	))
	if err := form.Run(); err != nil { return m } // esc = keep default
	return m
}

func selectTiersDetailed(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string, err error) {
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
	same := "(same as opus)"
	fh := DimStyle.Render("/ = search, ↑↓ = navigate, esc = go back")

	opus = defaultModel
	sonnet, haiku, subagent = defaultModel, defaultModel, defaultModel

	if huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Height(5).Title("Opus model (hard reasoning)").
			Description(fh).Options(opts()...).Value(&opus),
	)).Run() != nil { return }

	sOpts := opts(); sOpts = append(sOpts, huh.NewOption(same, ""))
	if huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Height(5).Title("Sonnet model (daily driver)").
			Description("Same as opus by default. " + fh).Options(sOpts...).Value(&sonnet),
	)).Run() != nil { return }

	hOpts := opts(); hOpts = append(hOpts, huh.NewOption(same, ""))
	if huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Height(5).Title("Haiku model (fast/cheap)").
			Description("Same as opus by default").Options(hOpts...).Value(&haiku),
	)).Run() != nil { return }

	subOpts := opts(); subOpts = append(subOpts, huh.NewOption(same, ""))
	if huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Height(5).Title("Subagent model").
			Description("Same as opus by default").Options(subOpts...).Value(&subagent),
	)).Run() != nil { return }

	return opus, sonnet, haiku, subagent, nil
}

func selectTiersFreeText() (opus, sonnet, haiku, subagent string, err error) {
	huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Opus model ID — PRIMARY").Placeholder("model-id").
			Validate(func(s string) error { if s == "" { return fmt.Errorf("required") }; return nil }).Value(&opus),
		huh.NewInput().Title("Sonnet model ID — optional").Placeholder("same as opus").Value(&sonnet),
		huh.NewInput().Title("Haiku model ID — optional").Placeholder("same as opus").Value(&haiku),
		huh.NewInput().Title("Subagent model ID — optional").Placeholder("same as opus").Value(&subagent),
	)).Run()
	return opus, sonnet, haiku, subagent, nil
}
