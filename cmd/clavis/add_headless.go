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
	if hasAddFlags(args) {
		runAddHeadless(args)
		return
	}
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
	contextWindow := fs.Int("context-window", 0, "Context window size in tokens")
	compactPct := fs.Int("compact-pct", 0, "Auto-compact percentage threshold")
	maxTokens := fs.Int("max-tokens", 0, "Max output tokens per request")
	effort := fs.String("effort", "", "Effort level (low|medium|high|xhigh|max|auto)")
	timeoutMs := fs.Int("timeout", 0, "API timeout in milliseconds")
	noThinking := fs.Bool("no-thinking", false, "Disable thinking tokens")
	noTraffic := fs.Bool("no-traffic", true, "Disable nonessential traffic")
	noBetas := fs.Bool("no-betas", true, "Disable experimental betas")
	minimal := fs.Bool("bare", false, "Minimal mode (--bare)")
	noAttr := fs.Bool("no-attribution", false, "Disable attribution header")
	customHeaders := fs.String("headers", "", "Custom headers (key:val\\nkey2:val2)")
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

	if *contextWindow > 0 {
		p.ContextWindow = contextWindow
	}
	if *compactPct > 0 {
		p.AutoCompactPct = compactPct
	}
	if *maxTokens > 0 {
		p.MaxOutputTokens = maxTokens
	}
	if *effort != "" {
		p.EffortLevel = *effort
	}
	if *timeoutMs > 0 {
		p.TimeoutMs = timeoutMs
	}
	if *noThinking {
		p.DisableThinking = true
	}
	if *customHeaders != "" {
		p.CustomHeaders = make(map[string]string)
		for _, line := range strings.Split(*customHeaders, "\n") {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				p.CustomHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	if err := profile.Save(p, *key); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Failed to save: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", name)))
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+name))
}
