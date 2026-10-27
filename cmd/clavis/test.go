package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mutex-solutions/clavis/internal/claude"
	"github.com/mutex-solutions/clavis/internal/profile"
	"github.com/mutex-solutions/clavis/internal/provider"
	"github.com/mutex-solutions/clavis/internal/ui"
)

// checkResult is one line of the test report.
type checkResult struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Info string `json:"info,omitempty"`
}

type testReport struct {
	OK      bool          `json:"ok"`
	Profile string        `json:"profile"`
	Checks  []checkResult `json:"checks"`
}

// runTest verifies a profile is set up correctly and runnable.
// Usage: clavis test <name> [--json]
func runTest(args []string) {
	asJSON := false
	var name string
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		default:
			if name == "" {
				name = a
			}
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "Usage: clavis test <name> [--json]")
		os.Exit(2)
	}

	report := testReport{OK: true, Profile: name, Checks: []checkResult{}}
	add := func(n string, ok bool, info string) {
		report.Checks = append(report.Checks, checkResult{Name: n, OK: ok, Info: info})
		if !ok {
			report.OK = false
		}
	}

	// 1. Profile loads and is structurally valid.
	p, err := profile.Load(name)
	if err != nil {
		add("profile loads", false, err.Error())
		finishTest(report, asJSON) // fatal — nothing else can run
		return
	}
	add("profile loads", true, "")

	if err := p.Validate(); err != nil {
		add("profile valid", false, err.Error())
	} else {
		add("profile valid", true, fmt.Sprintf("provider=%s model=%s", p.ProviderID, p.SonnetModel))
	}

	// 2. API key is present.
	apiKey, err := profile.LoadKey(name)
	if err != nil {
		add("api key present", false, err.Error())
		finishTest(report, asJSON)
		return
	}
	add("api key present", true, "")

	// 3. claude binary is installed.
	if path, err := claude.FindClaude(); err != nil {
		add("claude installed", false, "not found — install with: "+claude.InstallHint())
	} else {
		add("claude installed", true, path)
	}

	// 4. Each distinct model tier answers a live ping.
	seen := map[string]bool{}
	for _, m := range []string{p.SonnetModel, p.ResolvedOpus(), p.ResolvedHaiku(), p.ResolvedSubagent()} {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		if err := provider.Ping(p.BaseURL, apiKey, m); err != nil {
			add("ping "+m, false, err.Error())
		} else {
			add("ping "+m, true, "")
		}
	}

	finishTest(report, asJSON)
}

func finishTest(r testReport, asJSON bool) {
	if asJSON {
		out, _ := json.Marshal(r)
		fmt.Println(string(out))
	} else {
		fmt.Println()
		for _, c := range r.Checks {
			line := c.Name
			if c.Info != "" {
				line += "  " + ui.DimStyle.Render(c.Info)
			}
			if c.OK {
				fmt.Println("  " + ui.Success(line))
			} else {
				fmt.Println("  " + ui.Error(line))
			}
		}
		fmt.Println()
		if r.OK {
			fmt.Println("  " + ui.Success(fmt.Sprintf("Profile %q is ready", r.Profile)))
			fmt.Println(ui.NextStep("Launch it with:", "clavis "+r.Profile))
		} else {
			fmt.Println("  " + ui.Error(fmt.Sprintf("Profile %q has problems (see above)", r.Profile)))
		}
	}
	if !r.OK {
		os.Exit(1)
	}
}
