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
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	providerID := fs.String("provider", "", "Provider ID (deepseek, anthropic, zai, etc.)")
	model := fs.String("model", "", "Primary model (daily driver) — required")
	reasoningModel := fs.String("reasoning-model", "", "Hard-reasoning model (optional)")
	fastModel := fs.String("fast-model", "", "Fast/cheap model (optional)")
	subagentModel := fs.String("subagent-model", "", "Subagent model (optional)")
	baseURL := fs.String("base-url", "", "Base URL (required for custom providers)")
	alias := fs.String("alias", "", "Profile name (auto-generated if empty)")
	key := fs.String("key", "", "API key (insecure — prefer --key-file)")
	keyFile := fs.String("key-file", "", "Read API key from file (secure)")
	contextWindow := fs.Int("context-window", 0, "Context window size in tokens")
	compactPct := fs.Int("compact-pct", 0, "Auto-compact percentage threshold")
	maxTokens := fs.Int("max-tokens", 0, "Max output tokens per request")
	effort := fs.String("effort", "", "Effort level (low|medium|high|xhigh|max|auto)")
	timeoutMs := fs.Int("timeout", 0, "API timeout in ms (600000 default)")
	noThinking := fs.Bool("no-thinking", false, "Disable thinking tokens")
	minimal := fs.Bool("bare", false, "Minimal mode (--bare)")
	customHeaders := fs.String("headers", "", "Custom headers (key: val\\nkey2: val2)")

	err := fs.Parse(args)
	if err != nil && !hasAnyFlag(args) {
		runAdd()
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !hasAnyFlag(args) {
		runAdd()
		return
	}

	if *providerID == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--provider is required"))
		os.Exit(1)
	}
	if *model == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--model is required"))
		os.Exit(1)
	}

	apiKey := *key
	if *keyFile != "" {
		data, err := os.ReadFile(*keyFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Cannot read key file: %v", err)))
			os.Exit(1)
		}
		apiKey = strings.TrimSpace(string(data))
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--key or --key-file is required"))
		os.Exit(1)
	}

	prov, err := provider.Get(*providerID)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	url := *baseURL
	if url == "" {
		url = prov.BaseURL
	}
	if url == "" {
		fmt.Fprintln(os.Stderr, ui.Error("--base-url is required for custom providers"))
		os.Exit(1)
	}

	name := *alias
	if name == "" {
		name = profile.NextAlias(*providerID)
	}

	if profile.Exists(name) {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Profile %q already exists", name)))
		os.Exit(3)
	}

	noTraffic := prov.ID != "anthropic"
	noBetas := prov.ID != "anthropic"
	noAttr := prov.ID != "anthropic"

	fmt.Print(ui.DimStyle.Render("  Validating... "))
	if err := provider.Ping(url, apiKey, *model); err != nil {
		fmt.Println(ui.Error(fmt.Sprintf("Validation failed: %v", err)))
		os.Exit(1)
	}
	fmt.Println(ui.Success("Key valid"))

	p := &profile.Profile{
		Name:                       name,
		ProviderID:                 *providerID,
		BaseURL:                    url,
		SonnetModel:                *model,
		OpusModel:                  *reasoningModel,
		HaikuModel:                 *fastModel,
		SubagentModel:              *subagentModel,
		DisableNonessentialTraffic: &noTraffic,
		DisableExperimentalBetas:   &noBetas,
		DisableAttribution:         &noAttr,
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

	if err := profile.Save(p, apiKey); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Failed to save: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", name)))
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+name))
}

func hasAnyFlag(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "---") {
			return true
		}
	}
	return false
}
