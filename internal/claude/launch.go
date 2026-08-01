package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dinhlongviolin1/clavis/internal/profile"
)

// Launch builds the environment from a profile and spawns claude.
// It blocks until claude exits, then saves last-used state.
// Returns claude's exit code.
func Launch(p *profile.Profile, apiKey string, extraArgs []string) (int, error) {
	if err := p.Validate(); err != nil {
		return 1, fmt.Errorf("invalid profile: %w", err)
	}

	env, err := buildEnv(p, apiKey)
	if err != nil {
		return 1, fmt.Errorf("build env: %w", err)
	}

	// Write settings.json if attribution disabled
	if p.ShouldDisableAttribution() {
		if err := writeSettings(p); err != nil {
			return 1, fmt.Errorf("write settings: %w", err)
		}
	}

	// Build args
	args := buildArgs(p, extraArgs)

	// Find claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return 1, fmt.Errorf("claude binary not found in PATH — install Claude Code first")
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

	// Forward signals in background. Since claude is in its own process group
	// (Setpgid), we forward to the process group (negative PID) so that any
	// grandchildren claude spawns also receive the signal.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if cmd.Process != nil {
					_ = syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
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
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					// Signal death: return 128 + signal number (standard convention).
					return 128 + int(status.Signal()), nil
				}
				return status.ExitStatus(), nil
			}
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("claude exited with error: %w", err)
	}
	return 0, nil
}

// buildEnv constructs the full environment for the claude child process.
func buildEnv(p *profile.Profile, apiKey string) ([]string, error) {
	configDir := profile.ProfileDir(p.Name)

	// Build the set of vars we will explicitly set.
	set := map[string]string{
		"CLAUDE_CONFIG_DIR":              configDir,
		"ANTHROPIC_BASE_URL":             p.BaseURL,
		"ANTHROPIC_AUTH_TOKEN":           apiKey,
		"ANTHROPIC_MODEL":                p.SonnetModel,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": p.SonnetModel,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   p.ResolvedOpus(),
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  p.ResolvedHaiku(),
		"CLAUDE_CODE_SUBAGENT_MODEL":     p.ResolvedSubagent(),
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

	// Vars to always strip (auth leaks), plus everything we're about to set
	// (prevents duplicates — on Linux getenv returns the FIRST match).
	strip := map[string]bool{
		"ANTHROPIC_API_KEY":           true,
		"CLAUDE_CODE_OAUTH_TOKEN":     true,
		"CLAUDE_CREDENTIALS_JSON_B64": true,
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

// writeSettings writes a minimal settings.json for attribution control.
func writeSettings(p *profile.Profile) error {
	configDir := profile.ProfileDir(p.Name)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	settingsPath := filepath.Join(configDir, "settings.json")

	// Read existing settings if present (don't clobber)
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// Ensure env block exists
	envBlock, ok := settings["env"].(map[string]interface{})
	if !ok {
		envBlock = make(map[string]interface{})
	}
	envBlock["CLAUDE_CODE_ATTRIBUTION_HEADER"] = "0"
	settings["env"] = envBlock

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
