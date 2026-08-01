package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dinhlongviolin1/clavis/internal/profile"
	"github.com/dinhlongviolin1/clavis/internal/provider"
	"github.com/dinhlongviolin1/clavis/internal/ui"
)

// Exit codes are stable so agents can branch on them.
const (
	exitUsage      = 2 // bad flags / missing required flag
	exitProvider   = 3 // unknown provider or bad base URL
	exitExists     = 4 // profile name already taken
	exitValidation = 5 // key failed the provider ping
	exitSave       = 6 // couldn't write the profile to disk
)

// addResult is the machine-readable result emitted with --json.
type addResult struct {
	OK      bool   `json:"ok"`
	Profile string `json:"profile,omitempty"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
	// Field names the caller must fix, when the failure is a bad/missing flag.
	Field string `json:"field,omitempty"`
}

func runAddOrHeadless(args []string) {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	providerID := fs.String("provider", "", "Provider ID (deepseek, anthropic, zai, ...) — required")
	model := fs.String("model", "", "Primary model, the daily driver — required")
	reasoningModel := fs.String("reasoning-model", "", "Hard-reasoning model (falls back to --model)")
	fastModel := fs.String("fast-model", "", "Fast/cheap model (falls back to --model)")
	subagentModel := fs.String("subagent-model", "", "Subagent model (falls back to --model)")
	baseURL := fs.String("base-url", "", "Base URL (required only for --provider custom)")
	alias := fs.String("alias", "", "Profile name (auto-generated if empty)")
	key := fs.String("key", "", "API key (insecure on shared hosts — prefer --key-file)")
	keyFile := fs.String("key-file", "", "Read the API key from this file (secure)")
	contextWindow := fs.Int("context-window", 0, "Context window size in tokens")
	compactPct := fs.Int("compact-pct", 0, "Auto-compact threshold, 1-100 (%)")
	maxTokens := fs.Int("max-tokens", 0, "Max output tokens per request")
	effort := fs.String("effort", "", "Effort level: low|medium|high|xhigh|max|auto")
	timeoutMs := fs.Int("timeout", 0, "API timeout in ms (default 600000)")
	noThinking := fs.Bool("no-thinking", false, "Disable thinking tokens")
	minimal := fs.Bool("bare", false, "Minimal mode (fewer env vars set)")
	customHeaders := fs.String("headers", "", `Custom headers, "Key: val\nKey2: val2"`)
	asJSON := fs.Bool("json", false, "Emit a machine-readable JSON result (for agents)")
	fs.Usage = func() { printAddHelp(fs) }

	// -h/--help before parsing so it never looks like a parse error.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "-help" {
			printAddHelp(fs)
			return
		}
	}

	// No flags at all → launch the interactive wizard.
	if !hasAnyFlag(args) {
		runAdd()
		return
	}

	if err := fs.Parse(args); err != nil {
		// fs already printed the error + usage to stderr.
		os.Exit(exitUsage)
	}

	fail := func(code int, field, msg string) {
		if *asJSON {
			out, _ := json.Marshal(addResult{OK: false, Error: msg, Field: field})
			fmt.Fprintln(os.Stderr, string(out))
		} else {
			fmt.Fprintln(os.Stderr, ui.Error(msg))
		}
		os.Exit(code)
	}

	if *providerID == "" {
		fail(exitUsage, "provider", "--provider is required")
	}
	if *model == "" {
		fail(exitUsage, "model", "--model is required")
	}

	apiKey := *key
	if *keyFile != "" {
		data, err := os.ReadFile(*keyFile)
		if err != nil {
			fail(exitUsage, "key-file", fmt.Sprintf("cannot read key file: %v", err))
		}
		apiKey = strings.TrimSpace(string(data))
	}
	if apiKey == "" {
		fail(exitUsage, "key", "--key or --key-file is required")
	}

	prov, err := provider.Get(*providerID)
	if err != nil {
		fail(exitProvider, "provider", err.Error())
	}

	url := *baseURL
	if url == "" {
		url = prov.BaseURL
	}
	if url == "" {
		fail(exitProvider, "base-url", "--base-url is required for custom providers")
	}

	name := *alias
	if name == "" {
		name = profile.NextAlias(*providerID)
	}
	if err := profile.ValidName(name); err != nil {
		fail(exitUsage, "alias", err.Error())
	}
	if profile.Exists(name) {
		fail(exitExists, "alias", fmt.Sprintf("profile %q already exists", name))
	}

	noTraffic := prov.ID != "anthropic"
	noBetas := prov.ID != "anthropic"
	noAttr := prov.ID != "anthropic"

	if !*asJSON {
		fmt.Print(ui.DimStyle.Render("  Validating... "))
	}
	if err := provider.Ping(url, apiKey, *model); err != nil {
		fail(exitValidation, "key", fmt.Sprintf("key validation failed: %v", err))
	}
	if !*asJSON {
		fmt.Println(ui.Success("Key valid"))
	}

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
		fail(exitSave, "", fmt.Sprintf("failed to save: %v", err))
	}

	if *asJSON {
		out, _ := json.Marshal(addResult{OK: true, Profile: name, Path: profile.ProfileDir(name)})
		fmt.Println(string(out))
		return
	}
	fmt.Println(ui.Success(fmt.Sprintf("Profile %q saved", name)))
	fmt.Println(ui.NextStep("Launch it with:", "clavis "+name))
}

func printAddHelp(fs *flag.FlagSet) {
	fmt.Println(`clavis add — create a profile

Interactive:
  clavis add                     Launch the guided wizard (no flags)

Headless (for agents/CI — pass any flag to trigger this mode):
  clavis add --provider <id> --model <id> --key <k> [options]

Required:
  --provider <id>    Provider ID. See: clavis providers [--json]
  --model <id>       Primary model (the daily driver)
  --key <k> | --key-file <path>   API key (prefer --key-file)

Model tiers (all optional, each falls back to --model):
  --reasoning-model <id>   Hard-reasoning tier
  --fast-model <id>        Fast/cheap tier
  --subagent-model <id>    Subagent tier

Common options:
  --alias <name>     Profile name (auto-generated if omitted)
  --base-url <url>   Endpoint (required only for --provider custom)
  --effort <level>   low|medium|high|xhigh|max|auto
  --no-thinking      Disable thinking tokens
  --compact-pct <n>  Auto-compact threshold, 1-100
  --context-window <n> / --max-tokens <n> / --timeout <ms>
  --headers "K: v\nK2: v2"   Extra headers
  --bare             Minimal mode (fewer env vars)
  --json             Machine-readable result on stdout; errors as JSON on stderr

The key is always validated against the provider before the profile is saved.

Exit codes:
  0  saved         2  bad/missing flag     3  unknown provider / base URL
  4  name exists   5  key validation failed 6  save failed

Examples:
  clavis add --provider deepseek --model deepseek-chat --key-file ~/.dskey
  clavis add --provider custom --base-url https://api.x.com/anthropic \
    --model my-model --key sk-... --json`)
	_ = fs
}

func hasAnyFlag(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "---") {
			return true
		}
	}
	return false
}
