package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/ui"
)

// runGet implements `clavis get|show <name> [--json]`: read a profile's full
// config back. Agents use this for read-after-write / drift checks — it's the
// round-trip of what `add` wrote. The API key is never included; a `key_present`
// boolean reports whether one is stored.
func runGet(args []string) {
	var name string
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			fmt.Println("Usage: clavis get <name> [--json]")
			return
		default:
			if name == "" {
				name = a
			}
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "Usage: clavis get <name> [--json]")
		os.Exit(exitUsage)
	}

	p, err := profile.Load(name)
	if err != nil {
		if asJSON {
			out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
			fmt.Fprintln(os.Stderr, string(out))
		} else {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		}
		os.Exit(exitNotFound)
	}

	_, keyErr := profile.LoadKey(name)
	keyPresent := keyErr == nil

	if asJSON {
		// Round-trip the profile struct, then splice in resolved tiers + key
		// presence so agents don't have to re-derive fallbacks.
		raw, _ := json.Marshal(p)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		m["key_present"] = keyPresent
		m["resolved"] = map[string]string{
			"sonnet":   p.SonnetModel,
			"opus":     p.ResolvedOpus(),
			"haiku":    p.ResolvedHaiku(),
			"subagent": p.ResolvedSubagent(),
		}
		m["config_shared"] = !p.IsolatedConfig
		out, _ := json.Marshal(m)
		fmt.Println(string(out))
		return
	}

	// Human-readable.
	row := func(k, v string) {
		if v != "" {
			fmt.Printf("  %s %s\n", ui.DimStyle.Render(fmt.Sprintf("%-14s", k)), v)
		}
	}
	fmt.Println()
	fmt.Println("  " + ui.TitleStyle.Render(p.Name))
	row("provider", p.ProviderID)
	row("base url", p.BaseURL)
	row("model", p.SonnetModel)
	if p.OpusModel != "" {
		row("reasoning", p.OpusModel)
	}
	if p.HaikuModel != "" {
		row("fast", p.HaikuModel)
	}
	if p.SubagentModel != "" {
		row("subagent", p.SubagentModel)
	}
	if p.EffortLevel != "" {
		row("effort", p.EffortLevel)
	}
	if p.IsolatedConfig {
		row("config", "isolated")
	} else {
		row("config", "shared ~/.claude")
	}
	if keyPresent {
		row("key", "present")
	} else {
		row("key", ui.WarnStyle.Render("missing"))
	}
	fmt.Println()
}
