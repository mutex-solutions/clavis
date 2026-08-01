# Clavis — Specification (v2)

> Claude Code provider manager. Latin *clavis* = "key." Hold keys to many
> Claude backends, pick one, launch. Single static binary. Zero runtime.

---

## 1. Project Overview

**Clavis** is a CLI tool for managing multiple Anthropic-compatible API
providers for [Claude Code](https://code.claude.com). It stores API keys
securely on disk, presents a TUI selector to pick a profile at launch, and
forwards all arguments to the real `claude` binary.

**A profile is a config pack.** Clavis manages provider routing, model tier
mapping, and token management. It does NOT manage Claude Code settings,
hooks, permissions, or plugins. Those belong to the user's `~/.claude/`.

**Why not zsh aliases?** Keys end up in `.zshrc`. Switching means editing a
file. No discoverability. No validation. Clavis gives you a structured,
secure, keyboard-driven experience.

### Design goals

| Goal | How |
|---|---|
| Start instantly | Compiled binary, no interpreter warmup |
| Single binary | `go install` or `curl` a release |
| Keys never in shell history | Store in `~/.clavis/profiles/<name>/.env` (0600) |
| Keyboard-driven | Full TUI, no mouse needed |
| Passthrough to claude | `clavis maas -- --dangerously-skip-permissions` works |
| Provider catalog | Built-in list of known Anthropic-compatible endpoints |

---

## 2. CLI Surface

```
clavis [command] [args...]

Commands:
  start       Launch TUI selector, pick a profile, run claude (default)
  add         Interactive TUI to create a new profile
  list        Display all profiles as a table
  remove <n>  Delete a profile by name
  providers   Show built-in provider catalog
  validate <n> Ping the provider with a 1-token test
  version     Print version and exit

Flags:
  --help, -h     Show help
  --version, -v  Print version
```

### Default behavior (no subcommand)

Running `clavis` with no arguments opens the profile selector TUI. Selecting
a profile launches `claude` with that profile's configuration.

```
clavis                    # TUI picker → claude (interactive)
```

### Quick launch (profile name match)

If the first argument matches a profile name, skip the TUI entirely:

```
clavis maas                           # launch claude with maas profile
clavis maas "explain pkg/handler.go"  # forward prompt to claude
clavis deepseek -- "review this PR"   # explicit forwarding after --
```

**Unknown flags are auto-forwarded** when a profile name is given first.
Clavis only recognizes `--help` and `--version`. Everything else passes
through to `claude`:

```
clavis maas --dangerously-skip-permissions   # forwarded to claude
clavis maas --model claude-sonnet-5          # forwarded to claude
```

Use `--` to disambiguate if a future clavis flag conflicts:

```
clavis maas -- --model claude-sonnet-5
```

---

## 3. Data Model

### 3.1 Provider (built-in catalog)

A provider represents a known Anthropic-compatible API endpoint. Defined in a
JSON file embedded in the binary via `go:embed`.

```go
type Provider struct {
    ID          string         `json:"id"`          // unique slug, e.g. "deepseek"
    DisplayName string         `json:"displayName"` // "DeepSeek"
    BaseURL     string         `json:"baseUrl"`     // Anthropic-compatible endpoint
    Tier        string         `json:"tier"`        // "free" | "trial" | "paid"
    Auth        AuthConfig     `json:"auth"`
    Models      []ProviderModel `json:"models"`
    LastVerified string        `json:"lastVerified"` // ISO date
}

type AuthConfig struct {
    NeedsKey bool   `json:"needsKey"` // false = uses system keychain/OAuth
    KeyURL   string `json:"keyUrl"`   // where user gets a key
    EnvVar   string `json:"envVar"`   // usually "ANTHROPIC_AUTH_TOKEN"
}

type ProviderModel struct {
    ID        string   `json:"id"`        // model id sent to API
    Free      bool     `json:"free"`
    Context   int      `json:"context"`   // context window in tokens
    Note      string   `json:"note"`
    TierHints []string `json:"tierHints"` // suggested tiers: ["opus","sonnet"] or ["haiku","subagent"]
}
```

#### Built-in providers (v1)

Providers included in v1 are those proven to work in claudex.

| ID | Name | Base URL | Tier |
|---|---|---|---|
| `anthropic` | Anthropic (official) | `https://api.anthropic.com` | paid |
| `deepseek` | DeepSeek | `https://api.deepseek.com/anthropic` | paid |
| `zai` | Z.ai (Zhipu GLM) | `https://api.z.ai/api/anthropic` | free |
| `minimax` | MiniMax | `https://api.minimax.io/anthropic` | trial |
| `openrouter` | OpenRouter | `https://openrouter.ai/api` | free |
| `moonshot` | Moonshot Kimi | `https://api.moonshot.ai/anthropic` | paid |
| `custom` | Custom provider | *user-provided URL* | — |

#### Model tier hints in catalog

Each model in the catalog carries `tierHints` — suggested Claude Code tiers.
The add-profile wizard pre-selects based on these, but the user can override.

Example for DeepSeek:
```json
{
  "models": [
    { "id": "deepseek-v4-pro",   "tierHints": ["opus", "sonnet"],   "context": 128000 },
    { "id": "deepseek-v4-flash", "tierHints": ["haiku", "subagent"], "context": 128000 }
  ]
}
```

### 3.2 Profile (user-created)

A profile is a **config pack**: provider routing + model tiers + token
management + launch preferences. It does NOT manage Claude Code settings,
hooks, or permissions.

```go
type Profile struct {
    // ── Identity ──
    Name       string `json:"name"`        // user alias, e.g. "work", "deepseek-fast"
    ProviderID string `json:"provider_id"` // references Provider.ID
    BaseURL    string `json:"base_url"`    // endpoint URL (from provider, overridable)
    CreatedAt  string `json:"created_at"`  // ISO 8601
    UpdatedAt  string `json:"updated_at"`  // ISO 8601

    // ── Model Tiers (Claude Code 5-tier system) ──
    // Sonnet is required (daily driver). Others are optional — if empty,
    // they fall back to sonnet_model at launch.
    SonnetModel   string `json:"sonnet_model"`            // → ANTHROPIC_DEFAULT_SONNET_MODEL (REQUIRED)
    OpusModel     string `json:"opus_model,omitempty"`     // → ANTHROPIC_DEFAULT_OPUS_MODEL
    HaikuModel    string `json:"haiku_model,omitempty"`    // → ANTHROPIC_DEFAULT_HAIKU_MODEL
    SubagentModel string `json:"subagent_model,omitempty"` // → CLAUDE_CODE_SUBAGENT_MODEL

    // ── Token Budget ──
    ContextWindow   *int   `json:"context_window,omitempty"`    // → CLAUDE_CODE_AUTO_COMPACT_WINDOW
    AutoCompactPct  *int   `json:"auto_compact_pct,omitempty"`  // → CLAUDE_AUTOCOMPACT_PCT_OVERRIDE (only lowers threshold)
    MaxOutputTokens *int   `json:"max_output_tokens,omitempty"` // → CLAUDE_CODE_MAX_OUTPUT_TOKENS
    EffortLevel     string `json:"effort_level,omitempty"`      // → CLAUDE_CODE_EFFORT_LEVEL ("low"|"medium"|"high"|"xhigh"|"max"|"auto")
    DisableThinking bool   `json:"disable_thinking,omitempty"`  // → CLAUDE_CODE_DISABLE_THINKING (gateway compat)

    // ── Resilience ──
    TimeoutMs *int `json:"timeout_ms,omitempty"` // → API_TIMEOUT_MS (default 600000)

    // ── 3P Provider Hygiene ──
    // *bool: nil = use provider default (true for 3P, false for anthropic)
    //        true = force on, false = force off
    DisableNonessentialTraffic *bool `json:"disable_nonessential_traffic,omitempty"` // → "1"
    DisableExperimentalBetas   *bool `json:"disable_experimental_betas,omitempty"`   // → "1"
    DisableAttribution         *bool `json:"disable_attribution,omitempty"`          // → settings.json CLAUDE_CODE_ATTRIBUTION_HEADER=0

    // ── Launch Preferences ──
    MinimalMode   bool              `json:"minimal_mode,omitempty"`   // → pass --bare to claude
    CustomHeaders map[string]string `json:"custom_headers,omitempty"` // → ANTHROPIC_CUSTOM_HEADERS (literal newline-separated "Name: Value")
}
```

#### Tier fallback rules

At launch, any unset tier falls back to `sonnet_model`:

```
ANTHROPIC_MODEL                 = sonnet_model        (always)
ANTHROPIC_DEFAULT_SONNET_MODEL  = sonnet_model        (always, required)
ANTHROPIC_DEFAULT_OPUS_MODEL    = opus_model || sonnet_model
ANTHROPIC_DEFAULT_HAIKU_MODEL   = haiku_model || sonnet_model
CLAUDE_CODE_SUBAGENT_MODEL      = subagent_model || sonnet_model
```

The API key is stored **separately** in `~/.clavis/profiles/<name>/.env`,
never in the profile metadata file.

### 3.3 On-disk layout

```
~/.clavis/
├── profiles/
│   └── <name>/
│       ├── profile.json        # Profile config pack (no key)
│       └── .env                # contains CLAVIS_KEY=<api-key>  (chmod 0600)
└── state.json                  # last-used profile, preferences
```

Note: No `providers.json` in `~/.clavis/`. The catalog is embedded in the
binary and updated via releases. Custom providers are stored inside the
profile itself (provider_id="custom", base_url=user-provided).

---

## 4. Interactive UI (Inline Forms)

Clavis uses **inline terminal prompts**, NOT a fullscreen TUI. All UI renders
in the normal terminal scrollback — like tokamak's arrow menus or Claude
Code's own inline prompts. Built with [charmbracelet/huh](https://github.com/charmbracelet/huh).

### 4.1 Profile selector (`clavis` or `clavis start`)

```
$ clavis

  Pick a profile:
  > deepseek       DeepSeek (deepseek-v4-pro)
    zai            Z.ai (glm-4.7-flash)
    work           Anthropic (claude-sonnet-5)

  ↑/↓ move · enter launch · a add · d delete · q quit
```

- Arrow keys to navigate, enter to launch
- `a` jumps to add-profile flow
- `d` deletes highlighted profile (inline confirm)
- `q` or `esc` quits
- Last-used profile pre-selected

### 4.2 Add profile (`clavis add`)

Sequential inline forms with **dynamic tier selection**:

```
$ clavis add

  Profile name: deepseek_

  Provider:
  > DeepSeek                   [paid]
    Z.ai (Zhipu GLM)           [free]
    MiniMax                    [trial]
    OpenRouter                 [free]
    Moonshot Kimi              [paid]
    Anthropic (official)       [paid]
    Custom...

  Get an API key at: https://platform.deepseek.com/api_keys
  Paste your key: ********

  Sonnet model (daily driver) — REQUIRED:
  > deepseek-v4-pro [recommended]
    deepseek-v4-flash

  Opus model (hard reasoning) — optional:
  > deepseek-v4-pro [recommended]
    deepseek-v4-flash
    (same as sonnet)

  Haiku model (fast/cheap) — optional:
  > deepseek-v4-flash [recommended]
    deepseek-v4-pro
    (same as sonnet)

  Subagent model — optional:
  > deepseek-v4-flash [recommended]
    deepseek-v4-pro
    (same as sonnet)

  [x] Disable nonessential traffic
  [x] Disable experimental betas
  [ ] Minimal mode (--bare)
  [ ] Disable attribution header

  ✓ Key valid (200 OK, 142ms)
  ✓ Profile "deepseek" saved
```

- Each step renders inline, scrolls naturally
- Tier selects pre-populated from catalog `tierHints`
- Optional tiers default to "(same as sonnet)"
- Validation ping runs on confirm, shows result inline

### 4.3 List view (`clavis list`)

Styled table printed to stdout (lipgloss):

```
$ clavis list

  NAME         PROVIDER       SONNET MODEL       LAST USED
  deepseek     DeepSeek       deepseek-v4-pro    today
  zai          Z.ai           glm-4.7-flash      2 days ago
  work         Anthropic      claude-sonnet-5    3 hours ago

  3 profiles in ~/.clavis/profiles/
```

### 4.4 Non-TTY fallback

huh auto-detects non-TTY. In pipes/CI, selects become numbered prompts:

```
$ echo "1" | clavis add
  Provider:
    1) DeepSeek [paid]
    2) Z.ai [free]
  Select [default: 1]:
```

---

## 5. Launching Claude Code

When a profile is selected, clavis:

1. Loads the profile metadata from `profile.json`
2. Reads `~/.clavis/profiles/<name>/.env` to get the API key
3. Builds the environment:

```bash
# ── Always set ──
CLAUDE_CONFIG_DIR=$HOME/.clavis/profiles/<name>
ANTHROPIC_BASE_URL=<base_url>
ANTHROPIC_AUTH_TOKEN=$CLAVIS_KEY
ANTHROPIC_MODEL=<sonnet_model>
ANTHROPIC_DEFAULT_SONNET_MODEL=<sonnet_model>
ANTHROPIC_DEFAULT_OPUS_MODEL=<opus_model || sonnet_model>
ANTHROPIC_DEFAULT_HAIKU_MODEL=<haiku_model || sonnet_model>
CLAUDE_CODE_SUBAGENT_MODEL=<subagent_model || sonnet_model>

# ── Set if profile has values ──
CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=<auto_compact_pct>
CLAUDE_CODE_AUTO_COMPACT_WINDOW=<context_window>
CLAUDE_CODE_MAX_OUTPUT_TOKENS=<max_output_tokens>
CLAUDE_CODE_EFFORT_LEVEL=<effort_level>
CLAUDE_CODE_DISABLE_THINKING=1
API_TIMEOUT_MS=<timeout_ms>
ANTHROPIC_CUSTOM_HEADERS=<custom_headers>

# ── Default TRUE for 3P providers ──
CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1
```

4. **Explicitly unsets** (prevents leaks from user's shell):
```bash
unset ANTHROPIC_API_KEY          # prevents X-Api-Key header + subscription prompt
unset CLAUDE_CODE_OAUTH_TOKEN    # prevents OAuth interference
unset CLAUDE_CREDENTIALS_JSON_B64
```

5. If `disable_attribution` is true, writes a minimal `settings.json` into
   `CLAUDE_CONFIG_DIR`:
```json
{
  "env": {
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```
   This is the ONLY settings.json clavis writes. It does not manage hooks,
   permissions, plugins, or any other Claude Code settings.

6. If `minimal_mode` is true, appends `--bare` to the claude arguments.

7. **Launches `claude` as a child process** (`os/exec`, NOT `syscall.Exec`):
   - Passes through stdin/stdout/stderr (`cmd.Stdin = os.Stdin`, etc.)
   - Forwards all signals (Ctrl+C, SIGTERM)
   - Waits for claude to exit

8. On exit:
   - Saves "last used" timestamp to `state.json`
   - Propagates claude's exit code

### Why spawn+wait instead of exec

`syscall.Exec` replaces the clavis process — no code runs after claude
exits. This makes "last used profile" (the #1 UX feature for daily use)
impossible. `os/exec` spawn+wait keeps clavis alive to save state, show
errors, and propagate exit codes. Memory overhead (a few MB idle Go runtime)
is negligible.

### Signal handling (implementation note)

Go delivers SIGINT to the entire process group. When the user presses
Ctrl+C, both clavis AND claude receive it. Clavis must:
1. Catch SIGINT/SIGTERM via `signal.Notify` during `cmd.Wait()`
2. Forward to `cmd.Process.Signal()` if not already delivered
3. Continue waiting — do NOT exit early
4. Set `cmd.Stdin/Stdout/Stderr = os.Stdin/os.Stdout/os.Stderr` so claude
   owns the terminal

Clavis's TUI has already exited before claude spawns (selector closes →
terminal released → claude launched), so there is no TUI-in-TUI conflict.

### Why unset ANTHROPIC_API_KEY (not set to empty)

| Approach | Problem |
|---|---|
| `=""` (empty) | Still sends `X-Api-Key:` header. Unpredictable. |
| `=<same key>` | Sends duplicate auth headers (X-Api-Key + Bearer). Some gateways reject. |
| **unset** ✅ | Only `Authorization: Bearer` is sent. Clean. No subscription prompt. |

### CLAUDE_CONFIG_DIR: shared by default, isolation opt-in

**Current behavior (v0.1+):** profiles **share the user's `~/.claude`** by
default. `CLAUDE_CONFIG_DIR` is left inherited, so the user's MCP servers,
subagents, memory (`~/.claude/CLAUDE.md`), and settings are all present —
clavis only routes the provider/model via env vars.

Isolation is **opt-in**, per profile (`IsolatedConfig`, set in the wizard's
advanced step or `clavis add --isolated`) or per launch (`clavis <name>
--isolated` / `--shared`). When isolated, the profile gets its own
`CLAUDE_CONFIG_DIR` at `~/.clavis/profiles/<name>` with independent session
history and config.

> Earlier drafts forced isolation for every profile. That was reversed because
> it hid the user's entire Claude Code setup (MCP, agents, memory) behind a
> blank config, which is rarely what people want. Attribution control moved
> from writing a `settings.json` into the config dir to the
> `CLAUDE_CODE_ATTRIBUTION_HEADER` env var, so shared mode never mutates
> `~/.claude`.

---

## 6. Security

- API keys stored in `~/.clavis/profiles/<name>/.env`, mode `0600`
- `.env` files contain only `CLAVIS_KEY=<key>`, one line
- Keys never appear in `profile.json`
- Keys never passed via CLI arguments
- `clavis list` never displays keys
- `clavis validate` shows `✓` or `✗`, never echoes the key
- ANTHROPIC_API_KEY is explicitly unset at launch (prevents subscription key leaking to 3P endpoints)
- The `clavis` binary should be signed (goreleaser cosign)

---

## 7. Build & Release

### Tech stack

| Component | Choice |
|---|---|
| Language | Go 1.22+ |
| Inline forms | [huh](https://github.com/charmbracelet/huh) (selects, text inputs, checkboxes) |
| Styling | [lipgloss](https://github.com/charmbracelet/lipgloss) |
| CLI parsing | stdlib `flag` + subcommand routing |
| HTTP (validate) | stdlib `net/http` |
| Release | [goreleaser](https://goreleaser.com) |

### Project structure

```
clavis/
├── cmd/
│   └── clavis/
│       └── main.go              # entrypoint, subcommand routing
├── internal/
│   ├── claude/
│   │   └── launch.go            # env setup + os/exec launch
│   ├── profile/
│   │   └── profile.go           # CRUD for profiles on disk
│   ├── provider/
│   │   └── provider.go          # catalog loading, go:embed
│   └── ui/
│       ├── selector.go           # inline profile picker (huh.Select)
│       ├── add.go                # inline add-profile wizard (huh.Form)
│       └── styles.go             # lipgloss themes
├── embed/
│   └── providers.json            # built-in provider catalog (go:embed)
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
└── README.md
```

---

## 8. Non-Goals (v1)

- No background service or daemon
- No token counting or cost tracking
- No provider proxy/routing layer — relies on providers' own endpoints
- No macOS Keychain / Windows Credential Manager integration
- No multi-user support — single machine, single user
- No Claude Code settings/hooks/permissions management
- No settings.json sync or merge with global `~/.claude/`
- No `_SUPPORTED_CAPABILITIES` env var management (future)

---

## 9. Roadmap

### v0.1 (MVP)
- [ ] `clavis add` TUI wizard with dynamic tier selection
- [ ] `clavis start` TUI selector (default command)
- [ ] `clavis list` table view
- [ ] `clavis remove`
- [ ] `clavis providers`
- [ ] `clavis validate` — ping test
- [ ] Built-in catalog: anthropic, deepseek, zai, minimax, openrouter, moonshot, custom
- [ ] Profile → claude launch (spawn+wait, env setup, signal forwarding)
- [ ] Non-TTY fallback for scripts
- [ ] Last-used profile remembered and pre-selected

### v0.2
- [ ] Custom provider (`clavis add` → "Custom..." prompts for base URL)
- [ ] Profile export/import (share config without key)
- [ ] `clavis edit` — modify an existing profile
- [ ] Minimal mode (--bare) toggle per profile

### v0.3
- [ ] Goreleaser + Homebrew tap
- [ ] Default profile (skip selector)
- [ ] Additional providers (maas, jan) once proven

---

## Appendix A: Env Var Reference

Complete list of env vars clavis may set, with sources verified against
official Claude Code docs (code.claude.com/docs/en/env-vars) and production
usage in tokamak CLI.

| Env Var | Profile Field | Default | Notes |
|---|---|---|---|
| `CLAUDE_CONFIG_DIR` | (derived from name) | — | Per-profile session isolation |
| `ANTHROPIC_BASE_URL` | `base_url` | — | Provider endpoint |
| `ANTHROPIC_AUTH_TOKEN` | (from .env) | — | Bearer auth for 3P providers |
| `ANTHROPIC_MODEL` | `sonnet_model` | — | Primary session model |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | `opus_model` | falls back to sonnet | Hard reasoning tier |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | `sonnet_model` | — | Daily driver (required) |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | `haiku_model` | falls back to sonnet | Fast/cheap background |
| `CLAUDE_CODE_SUBAGENT_MODEL` | `subagent_model` | falls back to sonnet | Background agents |
| `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` | `auto_compact_pct` | unset | Only LOWERS threshold. 80 = compact at 80% |
| `CLAUDE_CODE_AUTO_COMPACT_WINDOW` | `context_window` | unset | Context window size for compaction math |
| `CLAUDE_CODE_MAX_OUTPUT_TOKENS` | `max_output_tokens` | unset | Default 32K for unknown models. Higher = less context before compaction |
| `CLAUDE_CODE_EFFORT_LEVEL` | `effort_level` | unset | String: low/medium/high/xhigh/max/auto |
| `CLAUDE_CODE_DISABLE_THINKING` | `disable_thinking` | unset | Gateway compat: omits thinking param. Not a true thinking kill switch |
| `API_TIMEOUT_MS` | `timeout_ms` | unset (600000) | Max 2147483647 |
| `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` | `disable_nonessential_traffic` | "1" for 3P | ANY non-empty value disables. Must unset to re-enable |
| `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` | `disable_experimental_betas` | "1" for 3P | Prevents beta headers causing 400s on 3P |
| `ANTHROPIC_CUSTOM_HEADERS` | `custom_headers` | unset | Format: "Header1: val1\nHeader2: val2" |

**Explicitly unset at launch:**
| Env Var | Why |
|---|---|
| `ANTHROPIC_API_KEY` | Prevents X-Api-Key header + subscription interference |
| `CLAUDE_CODE_OAUTH_TOKEN` | Prevents OAuth credential leaks |
| `CLAUDE_CREDENTIALS_JSON_B64` | Prevents credential leaks |

**Written to settings.json (if disable_attribution):**
| Key | Value | Why |
|---|---|---|
| `CLAUDE_CODE_ATTRIBUTION_HEADER` | `"0"` | Strips attribution from system prompt. Must go in settings.json env block, not shell env |

**Deprecated — do NOT use:**
| Env Var | Replacement |
|---|---|
| `ANTHROPIC_SMALL_FAST_MODEL` | `ANTHROPIC_DEFAULT_HAIKU_MODEL` |

**Exists but not in v1 scope:**
| Env Var | Notes |
|---|---|
| `ANTHROPIC_DEFAULT_FABLE_MODEL` | 6th model tier (Fable). New, edge-case. Add in v0.2+ if needed. |
| `ANTHROPIC_DEFAULT_{TIER}_MODEL_SUPPORTED_CAPABILITIES` | Declares model capabilities (e.g. `effort,thinking`). Useful for 3P models that don't support thinking. Future enhancement. |

**Does NOT exist (confirmed hallucination-free):**
| Env Var | Status |
|---|---|
| `ANTHROPIC_DEFAULT_MODEL` | Does not exist. Tokamak sets it but it's a no-op. `ANTHROPIC_MODEL` is the correct var. |
| `DISABLE_PROMPT_CACHING` | Does not exist as an env var. No way to disable caching client-side. |
