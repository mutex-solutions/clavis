package ui

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

// errBack signals that the user wants to go to the previous step.
var errBack = errors.New("go back")
var errCancel = errors.New("cancel")

// clearScreen clears the terminal and moves cursor to top-left.
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// runForm runs a huh form with wizard-consistent key handling:
//
//   - Ctrl+C always exits the whole process (130), on every step. huh runs in
//     raw mode (ISIG off), so a keyboard Ctrl+C never reaches an OS SIGINT
//     handler. We bind Ctrl+C to the form's Quit binding so the form quits
//     *cleanly* (bubbletea restores the terminal), then exit 130 after Run
//     returns to mirror the SIGINT a keyboard Ctrl+C would otherwise raise.
//   - Esc aborts the form (returns huh.ErrUserAborted), which callers treat as
//     "go back". By default huh disables its Back binding on the first field of
//     a group, so Esc would do nothing on our single-field steps; adding Esc to
//     the form-level Quit binding makes it work everywhere.
//
// Returns huh.ErrUserAborted on Esc, or the form error otherwise.
func runForm(form *huh.Form) error {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"))

	interrupted := false
	err := form.
		WithKeyMap(km).
		WithProgramOptions(stdinTTY(), tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+c" {
				interrupted = true
			}
			return msg
		})).
		Run()

	if interrupted {
		// The form already tore the terminal down above (clean quit); only now
		// is it safe to exit the process. Exiting inside the tea filter would
		// skip bubbletea's terminal restore and leave the shell in raw mode.
		os.Exit(130)
	}
	return err
}

type AddResult struct {
	Profile *profile.Profile
	APIKey  string
}

func AddProfile() (*AddResult, error) {
	// Ctrl+C (exit) and Esc (go back) are handled per-form inside runForm and
	// RunModelSelect — huh runs in raw mode, so an OS-level SIGINT handler here
	// would never see a keyboard Ctrl+C.
	var (
		providerID                                        string
		baseURL                                           string
		apiKey                                            string
		models                                            []provider.ModelInfo
		alias                                             string
		opusModel, sonnetModel, haikuModel, subagentModel string
		effort                                            string
		noThinking                                        bool
		compactPct                                        int
		isolated                                          bool
	)

	const (
		stepProvider = iota
		stepKey
		stepModels
		stepAlias
		stepModelSelect
		stepAdvanced
		stepConfirm
	)

	// isBack reports whether an error means "go to the previous step".
	isBack := func(err error) bool {
		return errors.Is(err, errBack) || errors.Is(err, huh.ErrUserAborted)
	}

	step := stepProvider
	for {
		clearScreen()
		switch step {
		case stepProvider:
			p, url, err := wizardProvider()
			if err != nil {
				if isBack(err) {
					return nil, errCancel
				} // esc at step 0 = cancel
				return nil, err
			}
			providerID, baseURL = p, url
			step = stepKey

		case stepKey:
			url, key, err := wizardKey(providerID, baseURL)
			if err != nil {
				if isBack(err) {
					step = stepProvider
					continue
				}
				return nil, err
			}
			baseURL, apiKey = url, key
			step = stepModels

		case stepModels:
			prov, _ := provider.Get(providerID)
			m, err := wizardModels(baseURL, prov.ModelsEndpoint(), apiKey)
			if err != nil {
				if isBack(err) {
					step = stepKey
					continue
				}
				return nil, err
			}
			models = m
			step = stepAlias

		case stepAlias:
			a, err := wizardAlias(providerID)
			if err != nil {
				// Skip the non-interactive fetch step when going back — jump
				// straight to the key input (the previous *visible* step).
				if isBack(err) {
					step = stepKey
					continue
				}
				return nil, err
			}
			alias = a
			step = stepModelSelect

		case stepModelSelect:
			o, s, h, sub, err := wizardModelSelect(models)
			if err != nil {
				if isBack(err) {
					step = stepAlias
					continue
				}
				return nil, err
			}
			opusModel, sonnetModel, haikuModel, subagentModel = o, s, h, sub
			step = stepAdvanced

		case stepAdvanced:
			e, nt, cp, iso, err := wizardAdvanced()
			if err != nil {
				if isBack(err) {
					step = stepModelSelect
					continue
				}
				return nil, err
			}
			effort, noThinking, compactPct, isolated = e, nt, cp, iso
			step = stepConfirm

		case stepConfirm:
			// Build the profile so we can show a full summary before saving.
			is3P := providerID != "anthropic"
			p := &profile.Profile{
				Name: alias, ProviderID: providerID, BaseURL: baseURL,
				OpusModel: opusModel, SonnetModel: sonnetModel,
				HaikuModel: haikuModel, SubagentModel: subagentModel,
				DisableNonessentialTraffic: &is3P,
				DisableExperimentalBetas:   &is3P,
				DisableAttribution:         &is3P,
				IsolatedConfig:             isolated,
			}
			if effort != "" {
				p.EffortLevel = effort
			}
			if noThinking {
				p.DisableThinking = true
			}
			if compactPct > 0 {
				p.AutoCompactPct = &compactPct
			}

			ok, err := wizardConfirm(p)
			if err != nil {
				if isBack(err) {
					step = stepAdvanced
					continue
				}
				return nil, err
			}
			if !ok {
				return nil, errCancel // user chose "Cancel" on the review screen
			}
			return &AddResult{Profile: p, APIKey: apiKey}, nil
		}
	}
}

// wizardConfirm shows a summary of the profile and asks the user to save.
// Returns (true, nil) to save, (false, nil) to cancel, or errBack on Esc.
func wizardConfirm(p *profile.Profile) (bool, error) {
	clearScreen()
	fmt.Println(TitleStyle.Render("Review profile"))
	fmt.Println()

	row := func(label, val string) {
		if val == "" {
			return
		}
		fmt.Printf("  %s %s\n", DimStyle.Render(fmt.Sprintf("%-16s", label)), val)
	}
	row("Name", p.Name)
	row("Provider", p.ProviderID)
	row("Base URL", p.BaseURL)
	row("Model (main)", p.SonnetModel)
	if p.OpusModel != "" && p.OpusModel != p.SonnetModel {
		row("Reasoning", p.OpusModel)
	}
	if p.HaikuModel != "" && p.HaikuModel != p.SonnetModel {
		row("Fast", p.HaikuModel)
	}
	if p.SubagentModel != "" && p.SubagentModel != p.SonnetModel {
		row("Subagent", p.SubagentModel)
	}
	if p.EffortLevel != "" && p.EffortLevel != "auto" {
		row("Effort", p.EffortLevel)
	}
	if p.DisableThinking {
		row("Thinking", "disabled")
	}
	if p.AutoCompactPct != nil {
		row("Auto-compact", fmt.Sprintf("%d%%", *p.AutoCompactPct))
	}
	if p.IsolatedConfig {
		row("Config", "isolated (own ~/.clavis dir)")
	} else {
		row("Config", "shared ~/.claude")
	}
	fmt.Println()

	var save bool
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Save this profile?").
			Description("The key is validated before saving. esc = go back.").
			Affirmative("Yes, save").Negative("Cancel").Value(&save),
	))); err != nil {
		return false, errBack
	}
	return save, nil
}

// wizardProvider asks the user to pick a provider from the catalog.
// Esc here aborts the whole wizard (handled by the caller).
func wizardProvider() (providerID, baseURL string, err error) {
	providers := provider.All()
	providerOptions := make([]huh.Option[string], 0, len(providers))
	for _, p := range providers {
		providerOptions = append(providerOptions, huh.NewOption(p.DisplayName, p.ID))
	}

	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Select a provider").
			Description(DimStyle.Render("esc = cancel")).Options(providerOptions...).Value(&providerID),
	))); err != nil {
		return "", "", err
	}

	prov, err := provider.Get(providerID)
	if err != nil {
		return "", "", err
	}
	return providerID, prov.BaseURL, nil
}

// wizardKey collects the base URL (for custom providers) and API key.
// Esc here returns errBack so the caller returns to provider selection.
func wizardKey(providerID, baseURL string) (outURL, apiKey string, err error) {
	prov, err := provider.Get(providerID)
	if err != nil {
		return "", "", err
	}
	outURL = baseURL

	var fields []huh.Field
	if providerID == "custom" {
		fields = append(fields,
			huh.NewInput().Title("Base URL").Description("Anthropic-compatible endpoint URL").
				Placeholder("https://api.example.com/anthropic").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("base URL is required for custom providers")
					}
					return nil
				}).Value(&outURL))
	}

	if prov.Auth.KeyURL != "" {
		fmt.Println(DimStyle.Render("  Get an API key at: " + prov.Auth.KeyURL))
	}

	fields = append(fields,
		huh.NewInput().Title("API key").EchoMode(huh.EchoModePassword).Placeholder("sk-...").
			Description(DimStyle.Render("esc = go back to provider")).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("API key is required")
				}
				return nil
			}).Value(&apiKey))

	if err := runForm(huh.NewForm(huh.NewGroup(fields...))); err != nil {
		return "", "", errBack
	}
	return outURL, apiKey, nil
}

func wizardModels(baseURL, modelsURL, apiKey string) ([]provider.ModelInfo, error) {
	tryFetch := func(mURL string) ([]provider.ModelInfo, error) {
		res, err := runWithSpinner("Fetching models…", func() (any, error) {
			return provider.FetchModels(baseURL, mURL, apiKey)
		})
		var models []provider.ModelInfo
		if res != nil {
			models, _ = res.([]provider.ModelInfo)
		}
		return models, err
	}

	models, err := tryFetch(modelsURL)
	if err == nil {
		return models, nil
	}

	// Fetch failed. Offer to point clavis at a custom model-list URL (the
	// messages endpoint and the /models endpoint often differ — e.g. Alibaba
	// MaaS serves models at /compatible-mode/v1/models), fall back to typing
	// model IDs by hand, or go back.
	for {
		fmt.Println(Warn(fmt.Sprintf("Could not fetch models: %v", err)))

		const (
			optURL    = "url"
			optManual = "manual"
			optBack   = "back"
		)
		choice := optURL
		if e := runForm(huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Model list couldn't be fetched").
				Description("The models endpoint may differ from the base URL. esc = go back.").
				Options(
					huh.NewOption("Provide a model-list URL and retry", optURL),
					huh.NewOption("Type model IDs manually", optManual),
					huh.NewOption("Go back", optBack),
				).Value(&choice),
		))); e != nil || choice == optBack {
			return nil, errBack
		}

		switch choice {
		case optManual:
			return nil, nil // signal manual entry downstream
		case optURL:
			var mURL string
			if e := runForm(huh.NewForm(huh.NewGroup(
				huh.NewInput().Title("Model-list URL").
					Description("Full URL to the model list, e.g. https://…/compatible-mode/v1/models").
					Placeholder("https://host/compatible-mode/v1/models").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("URL is required")
						}
						return nil
					}).Value(&mURL),
			))); e != nil {
				continue // esc on the URL input → back to the choice menu
			}
			models, err = tryFetch(strings.TrimSpace(mURL))
			if err == nil {
				fmt.Println(Success(fmt.Sprintf("Found %d models", len(models))))
				return models, nil
			}
			// fetch failed again → loop shows the error and the menu once more
		}
	}
}

func wizardAlias(providerID string) (string, error) {
	autoName := profile.NextAlias(providerID)
	var alias string
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Profile alias (optional)").
			Description("Skip for auto-name: " + CommandStyle.Render(autoName) + "  " + DimStyle.Render("esc = go back")).
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
	))); err != nil {
		return "", errBack
	}
	if alias == "" {
		alias = autoName
	}
	return alias, nil
}

func wizardModelSelect(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string, err error) {
	if len(models) == 0 {
		o, s, h, sub, e := selectTiersFreeText()
		return o, s, h, sub, e
	}

	var simple bool
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Simple model setup?").
			Description("One model for all tiers. esc = go back.").
			Affirmative("Yes, one model").Negative("No, per-tier").Value(&simple),
	))); err != nil {
		return "", "", "", "", errBack
	}

	if simple {
		m, err := selectOneModel(models)
		if err != nil {
			return "", "", "", "", err
		}
		return m, m, m, m, nil
	}
	return selectTiersDetailed(models)
}

func wizardAdvanced() (effort string, noThinking bool, compactPct int, isolated bool, err error) {
	var show bool
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Advanced configuration?").
			Description("Effort, thinking, compaction, config isolation. esc = go back.").
			Affirmative("Yes, customize").Negative("No, use defaults").Value(&show),
	))); err != nil {
		return "", false, 0, false, errBack // esc = back to model selection
	}
	if !show {
		return "", false, 0, false, nil // "No, use defaults" → proceed with defaults
	}

	// Show each setting on its own screen — one field per huh group renders
	// cleanly, whereas stacking several fields in one group misaligns them.

	// 1. Effort level
	effort = "auto"
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
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
	))); err != nil {
		return "", false, 0, false, errBack
	}

	// 2. Disable thinking tokens
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Disable thinking tokens?").
			Description("Omit thinking tokens. Needed for providers that don't support them. esc = go back.").
			Affirmative("Yes, disable").Negative("No").Value(&noThinking),
	))); err != nil {
		return "", false, 0, false, errBack
	}

	// 3. Auto-compact threshold
	var compactStr string
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Auto-compact threshold (%)").
			Description("Lower = compact sooner. Default: 80%. Empty = skip. esc = go back.").
			Placeholder("80").
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				v, err := strconv.Atoi(s)
				if err != nil || v < 1 || v > 100 {
					return fmt.Errorf("must be 1–100")
				}
				return nil
			}).Value(&compactStr),
	))); err != nil {
		return "", false, 0, false, errBack
	}
	if compactStr != "" {
		compactPct, _ = strconv.Atoi(compactStr)
	}

	// 4. Config isolation
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Isolate Claude config?").
			Description("No: share your ~/.claude (MCP servers, agents, memory). Yes: give this profile its own empty config. esc = go back.").
			Affirmative("Yes, isolate").Negative("No, share ~/.claude").Value(&isolated),
	))); err != nil {
		return "", false, 0, false, errBack
	}

	return effort, noThinking, compactPct, isolated, nil
}

func selectOneModel(models []provider.ModelInfo) (string, error) {
	return RunModelSelect("Select model", models, 5, "", nil)
}

func selectTiersDetailed(models []provider.ModelInfo) (opus, sonnet, haiku, subagent string, err error) {
	// Each tier floats the already-chosen models to the top (★) with the
	// most recent first, so the natural default is highlighted and one Enter
	// away — while any model is still reachable by scrolling or typing.
	opus, err = RunModelSelect("Opus model (hard reasoning)", models, 5, "", nil)
	if err != nil {
		return
	}

	sonnet, err = RunModelSelect("Sonnet model (daily driver)", models, 5, "", []string{opus})
	if err != nil {
		return
	}

	haiku, err = RunModelSelect("Haiku model (fast/cheap)", models, 5, "(default — use sonnet)", []string{sonnet, opus})
	if err != nil {
		return
	}

	subagent, err = RunModelSelect("Subagent model", models, 5, "(default — use sonnet)", []string{sonnet, opus, haiku})
	if err != nil {
		return
	}

	return opus, sonnet, haiku, subagent, nil
}

func selectTiersFreeText() (opus, sonnet, haiku, subagent string, err error) {
	var simple bool
	if runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Simple model setup?").
			Description("One model for all tiers.").
			Affirmative("Yes, one model").Negative("No, per-tier").Value(&simple),
	))) != nil {
		return "", "", "", "", errBack
	}

	if simple {
		if runForm(huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Model ID").Placeholder("model-id").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					return nil
				}).Value(&opus),
		))) != nil {
			return "", "", "", "", errBack
		}
		return opus, opus, opus, opus, nil
	}

	if runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Opus model ID — PRIMARY").Placeholder("model-id").
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("required")
				}
				return nil
			}).Value(&opus),
		huh.NewInput().Title("Sonnet model ID — optional").Placeholder("same as opus").Value(&sonnet),
		huh.NewInput().Title("Haiku model ID — optional").Placeholder("same as opus").Value(&haiku),
		huh.NewInput().Title("Subagent model ID — optional").Placeholder("same as opus").Value(&subagent),
	))) != nil {
		return "", "", "", "", errBack
	}
	return opus, sonnet, haiku, subagent, nil
}
