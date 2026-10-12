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

	// Build args
	args := buildArgs(p, extraArgs)

	// Find claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return 1, fmt.Errorf("Claude not found — install with: npm install -g @anthropic-ai/claude-code")
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
	set := map[string]string{
		"ANTHROPIC_BASE_URL":             p.BaseURL,
		"ANTHROPIC_AUTH_TOKEN":           apiKey,
		"ANTHROPIC_MODEL":                p.SonnetModel,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": p.SonnetModel,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   p.ResolvedOpus(),
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  p.ResolvedHaiku(),
		"CLAUDE_CODE_SUBAGENT_MODEL":     p.ResolvedSubagent(),
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

	if p.AutoCompactPct != nil {
		set["CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"] = fmt.Sprintf("%d", *p.AutoCompactPct)
	}
	if p.ContextWindow != nil {
		set["CLAUDE_CODE_AUTO_COMPACT_WINDOW"] = fmt.Sprintf("%d", *p.ContextWindow)
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

func buildEnv(p *profile.Profile, apiKey string) ([]string, error) {
	set := EnvVars(p, apiKey)

	// Vars to always strip (auth leaks), plus everything we're about to set
	// (prevents duplicates — on Linux getenv returns the FIRST match).
	strip := map[string]bool{
		"ANTHROPIC_API_KEY":           true,
		"CLAUDE_CODE_OAUTH_TOKEN":     true,
		"CLAUDE_CREDENTIALS_JSON_B64": true,
		// Always strip attribution: we own this decision per-profile, so an
		// inherited shell value must not leak through in shared-config mode.
		"CLAUDE_CODE_ATTRIBUTION_HEADER": true,
	}
	for k := range set {
		strip[k] = true
	}

	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if !strip[key] {
			env = append(env, e)
		}
	}

	for k, v := range set {
		env = append(env, k+"="+v)
	}

	return env, nil
}

// buildArgs constructs claude CLI arguments.
func buildArgs(p *profile.Profile, extraArgs []string) []string {
	var args []string
	if p.MinimalMode {
		args = append(args, "--bare")
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
		ClaudeArgs: buildArgs(p, extraArgs),
	}
}
