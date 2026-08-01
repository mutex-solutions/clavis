# Clavis — Agent Handoff

> Multi-provider Claude Code profile manager with TUI selector. Single Go binary. Zero runtime.

## Overview

Clavis stores API keys for multiple Anthropic-compatible providers, lets the user pick a profile from a TUI menu, and launches `claude` with the right environment variables. A profile is a **config pack** — provider routing, model tiers, token management. Clavis does NOT manage Claude Code settings, hooks, or permissions.

```
clavis              → TUI selector → pick profile → spawn claude
clavis maas "fix X" → skip TUI, forward args to claude
```

## Stack

- **Go 1.22+** with `charmbracelet/huh` (inline forms), `lipgloss` (styling)
- **Single binary** — no Node, no Python, no runtime
- **go:embed** for built-in provider catalog
- **Inline UI** — NOT fullscreen TUI. Renders in terminal scrollback like tokamak/Claude Code

## Project structure

```
cmd/clavis/main.go          → entrypoint, flag parsing, subcommand routing
internal/claude/            → env setup + os/exec spawn (NOT syscall.Exec)
internal/profile/           → CRUD for ~/.clavis/profiles/<name>/
internal/provider/          → catalog loader (go:embed)
internal/tui/               → bubbletea models (selector, add-wizard)
embed/providers.json        → built-in provider catalog (go:embed)
```

## Key design decisions

1. **Keys never in metadata.** `profile.json` has provider/model/config. The key lives alone in `profiles/<name>/.env` (chmod 0600).
2. **No proxy, no router.** We don't sit between Claude and the API. We set env vars and spawn.
3. **Unknown flags auto-forward.** Only `--help` and `--version` are claimed by clavis. Everything else after a profile name goes to `claude`.
4. **Non-TTY fallback.** In pipes/CI, fall back to plain text output.
5. **Spawn+wait, not exec.** `os/exec` keeps clavis alive after claude exits to save "last used" state, show errors, and propagate exit codes.
6. **Unset ANTHROPIC_API_KEY.** Never set to empty or duplicate — fully remove from child env to prevent X-Api-Key header and subscription prompts.
7. **5-tier model system.** Sonnet (required) + opus/haiku/subagent (optional, fall back to sonnet). Uses `ANTHROPIC_DEFAULT_{OPUS,SONNET,HAIKU}_MODEL` + `CLAUDE_CODE_SUBAGENT_MODEL`. Never use deprecated `ANTHROPIC_SMALL_FAST_MODEL`.
8. **CLAUDE_CONFIG_DIR per profile.** Full session isolation. Each provider gets its own history, resume state, config.
9. **No settings management.** Only writes `CLAUDE_CODE_ATTRIBUTION_HEADER=0` into settings.json if disable_attribution is set. Does not copy/merge/manage global ~/.claude/ settings.

## Workspace

```
~/.clavis/
├── profiles/<name>/
│   ├── profile.json          → config pack (no key)
│   └── .env                  → CLAVIS_KEY=<api-key>
└── state.json                → last-used profile, preferences
```

## Provider catalog (v1)

7 providers, all proven in claudex: anthropic, deepseek, zai, minimax, openrouter, moonshot, custom.
Models carry `tierHints` (e.g. `["opus","sonnet"]`) for wizard pre-selection.

## Build

```bash
make build        # → ./bin/clavis
make install      # → ~/.local/bin/clavis
make release      # goreleaser cross-compile + GitHub release
go test ./...     # run tests
```

## Git conventions

- Commit messages: lowercase, imperative (`add selector tui`)
- Feature branches off `main`, merge with PR
- NOTHING secret ever committed
