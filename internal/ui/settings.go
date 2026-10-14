package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mutex-solutions/clavis/internal/provider"
)

// The individual profile settings, each as a one-screen prompt. They are shared
// by the add wizard (internal/ui/add.go) and the profile editor
// (internal/ui/edit.go) so the two can never drift apart.
//
// Every prompt returns errBack when the user presses Esc, which callers treat
// as "go back". Ctrl+C exits the process (130) from inside runForm.

// Context-window picker sentinels. Real choices carry the token count itself,
// so any non-numeric value is one of these.
const (
	ctxDefault = "default"
	ctxCustom  = "custom"
)

// wizardContextWindow asks for the model's real context window — the setting
// that stops Claude Code assuming 200K for a model ID it doesn't recognise.
//
// `detected` is what the provider reported, best first; `current` is an
// existing value when editing. A nil return means "leave it unset".
func wizardContextWindow(detected []int, current *int) (*int, error) {
	type choice struct {
		tokens int
		note   string
	}
	var choices []choice
	seen := make(map[int]bool)
	push := func(n int, note string) {
		if n <= 0 || seen[n] {
			return
		}
		seen[n] = true
		choices = append(choices, choice{n, note})
	}

	// Order is also priority: the first entry is where the cursor starts.
	if current != nil {
		push(*current, "(current)")
	}
	for i, n := range detected {
		note := "(detected from provider)"
		if i > 0 {
			note = "(reported for another tier)"
		}
		push(n, note)
	}
	for _, n := range []int{128000, 200000, 256000, 512000, 1000000} {
		note := ""
		if n == 200000 {
			note = "(Claude Code's default guess)"
		}
		push(n, note)
	}

	opts := make([]huh.Option[string], 0, len(choices)+2)
	for _, c := range choices {
		label := FormatTokens(c.tokens)
		if c.note != "" {
			label += "  " + DimStyle.Render(c.note)
		}
		opts = append(opts, huh.NewOption(label, strconv.Itoa(c.tokens)))
	}
	opts = append(opts,
		huh.NewOption("Provider default — let Claude Code guess", ctxDefault),
		huh.NewOption("Custom…", ctxCustom),
	)

	// huh places the cursor on the option matching the bound value, so seeding
	// it with the first choice makes "current" (when editing) or "detected"
	// (when adding) the one-Enter default.
	chosen := ctxDefault
	if len(choices) > 0 {
		chosen = strconv.Itoa(choices[0].tokens)
	}

	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Context window").
			Description("Your model's real window, in tokens. Claude Code only knows claude-* IDs; for anything else it assumes 200K and says so. esc = go back.").
			Options(opts...).Value(&chosen),
	))); err != nil {
		return nil, errBack
	}

	switch chosen {
	case ctxDefault:
		return nil, nil
	case ctxCustom:
		return wizardContextCustom()
	default:
		n, err := strconv.Atoi(chosen)
		if err != nil || n <= 0 {
			return nil, nil
		}
		return &n, nil
	}
}

// wizardContextCustom collects a window the presets don't cover.
func wizardContextCustom() (*int, error) {
	var raw string
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Context window (tokens)").
			Description("128000, or 128k / 1M, or a bare 200 meaning 200K. esc = go back.").
			Placeholder("128000").
			Validate(func(s string) error {
				n, err := ParseTokenCount(s)
				if err != nil {
					return err
				}
				// A three-figure typo would declare a tiny window and compact
				// almost immediately, which reads as a hang rather than a bug.
				if n < 1000 {
					return fmt.Errorf("that's only %d tokens — did you mean %s?", n, FormatTokens(n*1000))
				}
				return nil
			}).Value(&raw),
	))); err != nil {
		return nil, errBack
	}
	n, err := ParseTokenCount(raw)
	if err != nil {
		return nil, errBack
	}
	return &n, nil
}

// detectedContextOptions returns the distinct context windows the provider
// reported for the models a profile will actually use — the main model first,
// then any tier reporting a different window, so the main one wins as the
// default. Empty when the provider reports nothing (the common case).
func detectedContextOptions(models []provider.ModelInfo, sonnet, opus, haiku, subagent string) []int {
	seen := make(map[int]bool)
	var out []int
	for _, id := range []string{sonnet, opus, haiku, subagent} {
		n := provider.ContextWindowFor(models, id)
		if n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// wizardEffort asks for the effort level. An empty `current` starts on "auto".
func wizardEffort(current string) (string, error) {
	effort := current
	if effort == "" {
		effort = "auto"
	}
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
		return "", errBack
	}
	return effort, nil
}

// wizardThinking asks whether to disable thinking tokens, starting from the
// current setting.
func wizardThinking(current bool) (bool, error) {
	noThinking := current
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Disable thinking tokens?").
			Description("Omit thinking tokens. Needed for providers that don't support them. esc = go back.").
			Affirmative("Yes, disable").Negative("No").Value(&noThinking),
	))); err != nil {
		return false, errBack
	}
	return noThinking, nil
}

// wizardCompactPct asks for the auto-compact threshold as a percentage of the
// context window. An empty answer leaves it unset (Claude Code's own default).
func wizardCompactPct(current *int) (*int, error) {
	raw := ""
	if current != nil {
		raw = strconv.Itoa(*current)
	}
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Auto-compact threshold (%)").
			Description("Lower = compact sooner. Default: 80%. Empty = leave unset. esc = go back.").
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
			}).Value(&raw),
	))); err != nil {
		return nil, errBack
	}
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, nil
	}
	return &v, nil
}

// wizardIsolation asks whether the profile gets its own CLAUDE_CONFIG_DIR
// instead of sharing the user's ~/.claude.
func wizardIsolation(current bool) (bool, error) {
	isolated := current
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Isolate Claude config?").
			Description("No: share your ~/.claude (MCP servers, agents, memory). Yes: give this profile its own empty config. esc = go back.").
			Affirmative("Yes, isolate").Negative("No, share ~/.claude").Value(&isolated),
	))); err != nil {
		return false, errBack
	}
	return isolated, nil
}

// wizardMinimal asks whether to launch claude with --bare.
func wizardMinimal(current bool) (bool, error) {
	minimal := current
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Minimal mode (--bare)?").
			Description("Yes: pass --bare to claude, for fewer built-in behaviours. esc = go back.").
			Affirmative("Yes, minimal").Negative("No, full").Value(&minimal),
	))); err != nil {
		return false, errBack
	}
	return minimal, nil
}

// wizardAPIKey collects a replacement API key. An empty answer keeps whatever
// is already stored, so the key is never required just to change something else.
func wizardAPIKey(keyPresent bool) (string, error) {
	var key string
	desc := "An empty answer keeps the key already stored."
	if !keyPresent {
		desc = "This profile has no key stored yet."
	}
	clearScreen()
	if err := runForm(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("New API key").EchoMode(huh.EchoModePassword).
			Placeholder("leave empty to keep the current key").
			Description(desc + " esc = go back.").Value(&key),
	))); err != nil {
		return "", errBack
	}
	return strings.TrimSpace(key), nil
}
