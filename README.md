# clavis

> 🔑 Claude Code provider manager. One binary, many backends.

[![CI](https://github.com/dinhlongviolin1/clavis/actions/workflows/ci.yml/badge.svg)](https://github.com/dinhlongviolin1/clavis/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dinhlongviolin1/clavis)](https://github.com/dinhlongviolin1/clavis/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Clavis** manages multiple Anthropic-compatible API providers for
[Claude Code](https://code.claude.com). Store API keys securely, pick a
profile, launch. Single static Go binary — no Node, no Python, no runtime.

```
$ clavis

  Pick a profile:
  > deepseek       DeepSeek (deepseek-chat)
    qwen           Custom (qwen3.7-max)
    work           Anthropic (claude-sonnet-5)

$ clavis deepseek "fix the auth bug"

  clavis deepseek  → deepseek
  model   deepseek-chat
  url     https://api.deepseek.com/anthropic
  config  default ~/.claude (your MCP servers, agents, memory)
  claude  fix the auth bug
```

## Why?

Claude Code works with any Anthropic-compatible API — DeepSeek, Z.ai,
MiniMax, OpenRouter, Moonshot, Qwen/Alibaba MaaS, and more. But switching
between them means juggling env vars in your shell. Keys leak into `.zshrc`.
No validation. No discoverability.

Clavis gives you a structured, secure, keyboard-driven way to manage all your
providers — and it keeps your existing Claude Code setup (MCP servers, agents,
memory, settings) intact while routing to whichever provider you choose.

## Features

- **5-tier model mapping** — map provider models to Claude Code's
  opus / sonnet / haiku / subagent tiers plus the primary `ANTHROPIC_MODEL`
- **Secure key storage** — keys in `.env` files (chmod 0600), never in metadata
- **Shares your `~/.claude` by default** — keep your MCP servers, agents, and
  memory; opt into an isolated config per-profile or per-launch
- **Live model discovery** — fetches the provider's real model list during setup
- **Key validation** — every profile is pinged before it's saved
- **Token management** — compaction control, effort level, output limits, timeouts
- **3P provider hygiene** — auto-disables telemetry, beta headers, and attribution
  for third-party providers
- **Agent-friendly** — headless `add` with flags, `--json` output, stable exit
  codes, `clavis test` for health checks
- **Self-updating** — `clavis update` pulls the latest release
- **Zero runtime** — single static binary for Linux, macOS, and Windows

## Install

### Quick install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/dinhlongviolin1/clavis/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/dinhlongviolin1/clavis/main/install.ps1 | iex
```

### From source (requires Go 1.23+)

```bash
go install github.com/dinhlongviolin1/clavis/cmd/clavis@latest
# or
git clone https://github.com/dinhlongviolin1/clavis.git
cd clavis && make install
```

### Manual

Download a build for your OS/arch from the
[releases page](https://github.com/dinhlongviolin1/clavis/releases) and put the
`clavis` binary on your `PATH`.

Once installed, keep it current with:

```bash
clavis update            # install the latest release
clavis update --check    # just report whether one is available
```

`clavis update` also refreshes any shell completions you've installed, so new
commands tab-complete after upgrading.

### Shell completion

```bash
clavis completion install zsh     # or bash / fish
```

### Uninstall

```bash
clavis completion uninstall       # remove installed completions (all shells)
rm "$(command -v clavis)"         # remove the binary
rm -rf ~/.clavis                  # optional: remove profiles + saved keys
```

> zsh caches completions in `~/.zcompdump`. After removing a completion, run
> `rm -f ~/.zcompdump* && exec zsh` so the old entry doesn't linger (otherwise
> `clavis<Tab>` may print "function definition file not found"). `clavis
> completion uninstall` reminds you of this.

## Quick Start

```bash
# Create a profile (interactive wizard: pick provider → key → models → review)
clavis add

# Launch with a profile
clavis deepseek

# Forward args to claude
clavis deepseek "explain this codebase"
clavis deepseek -- --dangerously-skip-permissions

# Check a profile is set up and runnable (pings every model tier)
clavis test deepseek

# List profiles
clavis list
```

## Commands

| Command | Description |
|---|---|
| `clavis` | Profile selector → launch claude |
| `clavis <name> [args...]` | Quick launch a profile (args after `--` go to claude) |
| `clavis add` | Create a new profile (interactive wizard) |
| `clavis add --provider … --model … --key …` | Create a profile headlessly (see `clavis add --help`) |
| `clavis list` / `ls` | List all profiles (`--json` for machine output) |
| `clavis remove <name>` / `rm` | Delete a profile (`-f` to skip confirm) |
| `clavis providers` | Show built-in provider catalog (`--json`) |
| `clavis validate <name>` | Ping a profile's provider |
| `clavis test <name>` | Full health check: config, key, claude, every model tier |
| `clavis update` | Update clavis to the latest release |
| `clavis completion <shell>` | Generate bash/zsh/fish completion |
| `clavis version` | Print version |

### Launch flags (before `--`)

| Flag | Effect |
|---|---|
| `--isolated` | Give this launch its own `CLAUDE_CONFIG_DIR` |
| `--shared` | Force sharing your `~/.claude` (default) |

Everything after `--`, or the first flag clavis doesn't recognise, is forwarded
verbatim to `claude`.

## Supported Providers

| Provider | Base URL |
|---|---|
| Anthropic | `api.anthropic.com` |
| DeepSeek | `api.deepseek.com/anthropic` |
| Z.ai (Zhipu GLM) | `api.z.ai/api/anthropic` |
| MiniMax | `api.minimax.io/anthropic` |
| OpenRouter | `openrouter.ai/api` |
| Moonshot Kimi | `api.moonshot.ai/anthropic` |
| Custom | *your endpoint* (e.g. Qwen/Alibaba MaaS) |

Any Anthropic-compatible endpoint works via the **Custom** provider — just
give it the base URL and key.

## For agents & CI

Clavis is scriptable end-to-end. The headless `add` accepts every field as a
flag, emits JSON, and uses stable exit codes:

```bash
clavis add --json \
  --provider deepseek \
  --model deepseek-chat \
  --reasoning-model deepseek-reasoner \
  --key-file ~/.secrets/deepseek \
  --alias ds
# → {"ok":true,"profile":"ds","path":"/home/you/.clavis/profiles/ds"}

clavis test ds --json
# → {"ok":true,"profile":"ds","checks":[...]}
```

Exit codes for `add`: `0` saved · `2` bad/missing flag · `3` unknown provider /
base URL · `4` name exists · `5` key validation failed · `6` save failed.

See `clavis add --help` for the full flag reference.

## How It Works

Clavis is a **launcher**, not a proxy. It sets environment variables and spawns
`claude` as a child process:

```
~/.clavis/
├── profiles/
│   └── deepseek/
│       ├── profile.json    # config pack (no secrets)
│       └── .env            # CLAVIS_KEY=sk-...  (0600)
└── state.json              # last-used profile
```

At launch, clavis:

1. Reads the profile config + API key.
2. Sets `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, the model tiers,
   compaction, and hygiene vars.
3. **Unsets** `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, and other
   credential vars so nothing from your shell leaks through.
4. Leaves `CLAUDE_CONFIG_DIR` **inherited** (shared `~/.claude`) unless the
   profile or launch opts into isolation.
5. Spawns `claude` with signal forwarding, waits for exit, saves state.

## Config isolation

By default a profile **shares your real `~/.claude`** — your MCP servers,
subagents, memory, and settings all work exactly as they do normally; clavis
just points Claude Code at a different provider/model.

If you'd rather a profile run in a clean, isolated environment (its own
`CLAUDE_CONFIG_DIR` under `~/.clavis/profiles/<name>`), enable it:

- in the wizard's advanced step ("Isolate Claude config?"),
- headlessly with `clavis add --isolated`,
- or per-launch with `clavis <name> --isolated` (or `--shared` to override).

## Model Tiers

Claude Code routes different tasks to different model tiers:

| Tier | Env Var | Use |
|---|---|---|
| **Sonnet** (required) | `ANTHROPIC_DEFAULT_SONNET_MODEL` | Daily driver, main conversation |
| Opus | `ANTHROPIC_DEFAULT_OPUS_MODEL` | Hard reasoning, architecture |
| Haiku | `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Fast background tasks |
| Subagent | `CLAUDE_CODE_SUBAGENT_MODEL` | Background agents |

Optional tiers fall back to sonnet if not set. The primary `ANTHROPIC_MODEL` is
set to the sonnet model.

## Development

```bash
make build      # → bin/clavis
make install    # → ~/.local/bin/clavis
make test       # go test ./...
make lint       # go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## License

[MIT](LICENSE) © 2026 Long Nguyen
