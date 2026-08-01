package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
	"github.com/dinhlongviolin1/clavis/internal/ui"
)

func runAddOrHeadless(args []string) {
	// If any flags are present, use headless mode.
	if hasAddFlags(args) {
		runAddHeadless(args)
		return
	}
	// Otherwise, interactive TUI wizard.
	runAdd()
}

func hasAddFlags(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}

func runAddHeadless(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	providerID := fs.String("provider", "", "Provider ID (deepseek, anthropic, zai, etc.)")
	key := fs.String("key", "", "API key for the provider")
	baseURL := fs.String("url", "", "Base URL (required for custom providers)")
	alias := fs.String("alias", "", "Profile name (auto-generated if empty)")
	sonnet := fs.String("sonnet", "", "Sonnet model ID (required)")
	opus := fs.String("opus", "", "Opus model ID (optional)")
	haiku := fs.String("haiku", "", "Haiku model ID (optional)")
	subagent := fs.String("subagent", "", "Subagent model ID (optional)")
	noTraffic := fs.Bool("no-traffic", true, "Disable nonessential traffic")
	noBetas := fs.Bool("no-betas", true, "Disable experimental betas")
	minimal := fs.Bool("bare", false, "Minimal mode (--bare)")
	noAttr := fs.Bool("no-attribution", false, "Disable attribution header")
	fs.Parse(args)

	if *providerID == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--provider is required"))
		os.Exit(1)
	}
	if *key == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--key is required"))
		os.Exit(1)
	}
	if *sonnet == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--sonnet is required"))
		os.Exit(1)
	}

	prov, err := provider.Get(*providerID)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	url := prov.BaseURL
	if *baseURL != "" {
		url = *baseURL
	}
	if url == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--url is required for custom providers"))
		os.Exit(1)
	}

	name := *alias
	if name == "" {
		name = profile.NextAlias(*providerID)
	}

	// Validate key
	fmt.Print(ui.DimStyle.Render("  Validating... "))
	if err := provider.Ping(url, *key, *sonnet); err != nil {
		fmt.Println(ui.Warn(fmt.Sprintf("Could not verify key: %v", err)))
	} else {
		fmt.Println(ui.Success("Key valid"))
	}

	p := &profile.Profile{
		Name:                       name,
		ProviderID:                 *providerID,
		BaseURL:                    url,
		SonnetModel:                *sonnet,
		OpusModel:                  *opus,
		HaikuModel:                 *haiku,
		SubagentModel:              *subagent,
		DisableNonessentialTraffic: noTraffic,
		DisableExperimentalBetas:   noBetas,
		DisableAttribution:         noAttr,
		MinimalMode:                *minimal,
	}

	if err := profile.Save(p, *key); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Failed to save: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", name)))
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+name))
}
