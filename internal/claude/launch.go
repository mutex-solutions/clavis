package claude

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mutex-solutions/clavis/internal/profile"
)

// Launch builds the environment from a profile and spawns claude.
// It blocks until claude exits, then saves last-used state.
// Returns claude's exit code.
func Launch(p *profile.Profile, apiKey string, extraArgs []string) (int, error) {
	if err := p.Validate(); err != nil {
		return 1, fmt.Errorf("Invalid profile: %w", err)
	}

	env, err := buildEnv(p, apiKey)
	if err != nil {
		return 1, fmt.Errorf("build env: %w", err)
	}

	// Attribution is disabled via the CLAUDE_CODE_ATTRIBUTION_HEADER env var
	// (set in buildEnv), not a settings.json file — so we never write into the
	// user's real ~/.claude when running in shared-config mode.

	// Build args. The picker settings are generated here rather than in
	// PlanFor because they need the provider's live catalogue and the API key.
	settingsPath := ""
	if !hasSettingsFlag(extraArgs) {
		settingsPath = WriteModelPickerSettings(p, apiKey)
	}
	args := buildArgs(p, extraArgs, settingsPath)

	// Find claude binary. If it is missing, offer the platform installer rather
	// than failing with a command the user then has to look up.
	claudePath, err := FindClaude()
	if err != nil {
		claudePath, err = OfferInstall()
		if err != nil {
			return 1, err
		}
	}

	cmd := exec.Command(claudePath, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Isolate claude in its own process group so terminal signals (Ctrl+C)
	// don't double-deliver. On parent death, the kernel sends SIGTERM.
	cmd.SysProcAttr = setPlatformProcAttr()

	// Signal forwarding: catch SIGINT/SIGTERM and forward to child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start claude: %w", err)
	}

	// Forward signals in background to the child (and its process group where
	// the platform supports it, so grandchildren also receive the signal).
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if cmd.Process != nil {
					forwardSignal(cmd.Process, sig)
				}
			case <-done:
				return
			}
		}
	}()

	err = cmd.Wait()
	close(done)

	// Save last-used state
	profile.SaveLastUsed(p.Name)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Unwrap to get the platform-specific exit status.
			if code, ok := exitCode(exitErr.Sys()); ok {
				return code, nil
			}
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("claude exited with error: %w", err)
	}
	return 0, nil
}

// buildEnv constructs the full environment for the claude child process.
// EnvVars returns the environment variables clavis sets for a profile — just
// the clavis-managed keys, not the inherited environment. Callers that want to
// run claude themselves (e.g. an agent runner) can apply these directly.
// The API key is included as ANTHROPIC_AUTH_TOKEN.
func EnvVars(p *profile.Profile, apiKey string) map[string]string {
	set := map[string]string{}
	if p.AuthMode == profile.AuthOAuth {
		// A setup token authenticates a subscription against Anthropic itself.
		// It must not be pointed at a gateway, and it takes precedence over the
		// keychain, so the base URL is deliberately left unset.
		set["CLAUDE_CODE_OAUTH_TOKEN"] = apiKey
	} else {
		set["ANTHROPIC_BASE_URL"] = p.BaseURL
		set["ANTHROPIC_AUTH_TOKEN"] = apiKey
	}
	// A setup-token profile may bind no model at all, in which case Claude's
	// own default applies and none of the tier vars are set.
	if p.SonnetModel != "" {
		set["ANTHROPIC_MODEL"] = p.SonnetModel
		set["ANTHROPIC_DEFAULT_SONNET_MODEL"] = p.SonnetModel
		set["ANTHROPIC_DEFAULT_OPUS_MODEL"] = p.ResolvedOpus()
		set["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = p.ResolvedHaiku()
		set["ANTHROPIC_DEFAULT_FABLE_MODEL"] = p.ResolvedFable()
		set["CLAUDE_CODE_SUBAGENT_MODEL"] = p.ResolvedSubagent()
	}

	// Config dir: isolated profiles get their own; shared profiles inherit the
	// user's default ~/.claude (we don't set CLAUDE_CONFIG_DIR at all).
	if dir, isolated := p.ConfigDir(); isolated {
		set["CLAUDE_CONFIG_DIR"] = dir
	}

	// Attribution header (disabled for 3P by default) — via env, not a file.
	if p.ShouldDisableAttribution() {
		set["CLAUDE_CODE_ATTRIBUTION_HEADER"] = "0"
	}

	// The context window, then the two compaction knobs measured against it.
	//
	// These are three different things and only the order above composes
	// correctly. CLAUDE_CODE_MAX_CONTEXT_TOKENS declares how big the model's
	// window actually is — what 100% means. CLAUDE_AUTOCOMPACT_PCT_OVERRIDE is a
	// fraction of that. CLAUDE_CODE_AUTO_COMPACT_WINDOW is an absolute token
	// count, and Claude Code clamps it to the window it believes the model has.
	//
	// For a non-claude-* ID that believed window defaults to 200K, so a large
	// compaction window on its own is a trap: it quiets the "unrecognised
	// model" notice while the session is still capped at 200K, and requests get
	// rejected by the API long before the configured boundary is reached. Set
	// MaxContextTokens for any model whose real window isn't 200K.
	if p.MaxContextTokens != nil {
		set["CLAUDE_CODE_MAX_CONTEXT_TOKENS"] = fmt.Sprintf("%d", *p.MaxContextTokens)
	}
	if p.AutoCompactPct != nil {
		set["CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"] = fmt.Sprintf("%d", *p.AutoCompactPct)
	}
	if p.AutoCompactWindow != nil {
		set["CLAUDE_CODE_AUTO_COMPACT_WINDOW"] = fmt.Sprintf("%d", *p.AutoCompactWindow)
	}
	if p.MaxOutputTokens != nil {
		set["CLAUDE_CODE_MAX_OUTPUT_TOKENS"] = fmt.Sprintf("%d", *p.MaxOutputTokens)
	}
	if p.EffortLevel != "" {
		set["CLAUDE_CODE_EFFORT_LEVEL"] = p.EffortLevel
	}
	if p.DisableThinking {
		set["CLAUDE_CODE_DISABLE_THINKING"] = "1"
	}
	if p.TimeoutMs != nil {
		set["API_TIMEOUT_MS"] = fmt.Sprintf("%d", *p.TimeoutMs)
	}
	if p.ShouldDisableNonessentialTraffic() {
		set["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	}
	if p.ShouldDisableExperimentalBetas() {
		set["CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"] = "1"
	}
	if len(p.CustomHeaders) > 0 {
		var parts []string
		for k, v := range p.CustomHeaders {
			// Sanitize: strip newlines/carriage returns to prevent header injection.
			k = strings.NewReplacer("\n", "", "\r", "").Replace(k)
			v = strings.NewReplacer("\n", "", "\r", "").Replace(v)
			parts = append(parts, fmt.Sprintf("%s: %s", k, v))
		}
		set["ANTHROPIC_CUSTOM_HEADERS"] = strings.Join(parts, "\n")
	}
	return set
}

// managedEnv is every variable clavis can set. It doubles as the strip list so
// that switching a profile's auth mode cannot leave the previous mode's
// credential in the environment.
var managedEnv = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_FABLE_MODEL",
	"ANTHROPIC_CUSTOM_HEADERS",
	"CLAUDE_CODE_SUBAGENT_MODEL",
	"CLAUDE_CODE_OAUTH_TOKEN",
	"CLAUDE_CREDENTIALS_JSON_B64",
	"CLAUDE_CODE_ATTRIBUTION_HEADER",
	"CLAUDE_CODE_MAX_CONTEXT_TOKENS",
	"CLAUDE_AUTOCOMPACT_PCT_OVERRIDE",
	"CLAUDE_CODE_AUTO_COMPACT_WINDOW",
	"CLAUDE_CODE_MAX_OUTPUT_TOKENS",
	"CLAUDE_CODE_EFFORT_LEVEL",
	"CLAUDE_CODE_DISABLE_THINKING",
	"API_TIMEOUT_MS",
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
	"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
}

func buildEnv(p *profile.Profile, apiKey string) ([]string, error) {
	set := EnvVars(p, apiKey)

	// cleanEnv drops every var clavis may ever set, not just the ones this
	// profile sets. A leftover ANTHROPIC_AUTH_TOKEN from the shell would
	// otherwise ride along into an OAuth session and take precedence over the
	// setup token, and an inherited ANTHROPIC_BASE_URL would send it somewhere
	// it must not go.
	env := cleanEnv()

	for k, v := range set {
		env = append(env, k+"="+v)
	}

	return env, nil
}

// cleanEnv is os.Environ without any variable clavis manages. It is what the
// launch path starts from, and what `claude setup-token` runs under so the
// shell's gateway variables cannot confuse the flow.
func cleanEnv() []string {
	strip := map[string]bool{}
	for _, k := range managedEnv {
		strip[k] = true
	}
	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if !strip[key] {
			env = append(env, e)
		}
	}
	return env
}

// buildArgs constructs claude CLI arguments.
func buildArgs(p *profile.Profile, extraArgs []string, settingsPath string) []string {
	var args []string
	if p.MinimalMode {
		args = append(args, "--bare")
	}
	if settingsPath != "" {
		args = append(args, "--settings", settingsPath)
	}
	args = append(args, extraArgs...)
	return args
}

// Plan describes exactly what a launch will do, for a pre-flight banner.
type Plan struct {
	Provider   string   // provider ID
	BaseURL    string   // routed endpoint
	Model      string   // primary (sonnet) model
	ConfigDir  string   // "" means the user's default ~/.claude
	Isolated   bool     // whether a per-profile config dir is used
	ClaudeArgs []string // args forwarded to the claude binary
}

// PlanFor returns what Launch would do with this profile and extra args,
// so callers can show it before spawning claude.
func PlanFor(p *profile.Profile, extraArgs []string) Plan {
	if len(extraArgs) > 0 && extraArgs[0] == "--" {
		extraArgs = extraArgs[1:]
	}
	dir, isolated := p.ConfigDir()
	return Plan{
		Provider:   p.ProviderID,
		BaseURL:    p.BaseURL,
		Model:      p.SonnetModel,
		ConfigDir:  dir,
		Isolated:   isolated,
		ClaudeArgs: buildArgs(p, extraArgs, ""),
	}
}
