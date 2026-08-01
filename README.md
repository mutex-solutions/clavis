# clavis

> 🔑 Claude Code provider manager. One binary, many backends.

**Clavis** manages multiple Anthropic-compatible API providers for
[Claude Code](https://code.claude.com). Store API keys securely, pick a
profile, launch. Single static Go binary — no Node, no Python, no runtime.

```
$ clavis

  Pick a profile:
  > deepseek       DeepSeek (deepseek-v4-pro)
    zai            Z.ai (glm-4.7-flash)
    work           Anthropic (claude-sonnet-5)

$ clavis deepseek "fix the auth bug"
  → launching claude via deepseek (deepseek-v4-pro)
```

## Why?

Claude Code works with any Anthropic-compatible API — DeepSeek, Z.ai,
MiniMax, OpenRouter, Moonshot, and more. But switching between them means
juggling env vars in your shell. Keys leak into `.zshrc`. No validation.
No discoverability.

Clavis gives you a structured, secure, keyboard-driven way to manage
all your providers.

## Features

- **5-tier model mapping** — map provider models to Claude Code's opus/sonnet/haiku/subagent tiers
- **Secure key storage** — keys in `.env` files (chmod 0600), never in metadata
- **Session isolation** — each profile gets its own `CLAUDE_CONFIG_DIR`
- **Token management** — compaction control, effort level, output limits, timeouts
- **3P provider hygiene** — auto-disables telemetry, beta headers, attribution for third-party providers
- **Provider catalog** — 7 built-in providers with model tier hints
- **Zero runtime** — single ~8MB static binary

## Install

```bash
# From source (requires Go 1.22+)
go install github.com/dinhlongviolin1/clavis/cmd/clavis@latest

# Or build locally
git clone https://github.com/dinhlongviolin1/clavis.git
cd clavis && make install
```

## Quick Start

```bash
# Create a profile (interactive wizard)
clavis add

# Launch with a profile
clavis deepseek

# Forward args to claude
clavis deepseek "explain this codebase"
clavis deepseek -- --dangerously-skip-permissions

# List profiles
clavis list

# Validate a key
clavis validate deepseek
```

## Commands

| Command | Description |
|---|---|
| `clavis` | Profile selector → launch claude |
| `clavis <name> [args...]` | Quick launch a profile |
| `clavis add` | Create a new profile (interactive) |
| `clavis list` | List all profiles |
| `clavis remove <name>` | Delete a profile |
| `clavis providers` | Show built-in provider catalog |
| `clavis validate <name>` | Ping a profile's provider |
| `clavis version` | Print version |

## Supported Providers

| Provider | Base URL | Tier |
|---|---|---|
| Anthropic | `api.anthropic.com` | paid |
| DeepSeek | `api.deepseek.com/anthropic` | paid |
| Z.ai (Zhipu GLM) | `api.z.ai/api/anthropic` | free |
| MiniMax | `api.minimax.io/anthropic` | trial |
| OpenRouter | `openrouter.ai/api` | free |
| Moonshot Kimi | `api.moonshot.ai/anthropic` | paid |
| Custom | *your endpoint* | — |

## How It Works

Clavis is a **launcher**, not a proxy. It sets environment variables and
spawns `claude` as a child process:

```
~/.clavis/
├── profiles/
│   └── deepseek/
│       ├── profile.json    # config pack (no secrets)
│       └── .env            # CLAVIS_KEY=sk-...  (0600)
└── state.json              # last-used profile
```

At launch, clavis:
1. Reads the profile config + API key
2. Sets `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, model tiers, compaction, hygiene vars
3. **Unsets** `ANTHROPIC_API_KEY` and `CLAUDE_CODE_OAUTH_TOKEN` (prevents leaks)
4. Sets `CLAUDE_CONFIG_DIR` for per-profile session isolation
5. Spawns `claude` with signal forwarding, waits for exit, saves state

## Model Tiers

Claude Code routes different tasks to different model tiers:

| Tier | Env Var | Use |
|---|---|---|
| **Sonnet** (required) | `ANTHROPIC_DEFAULT_SONNET_MODEL` | Daily driver, main conversation |
| Opus | `ANTHROPIC_DEFAULT_OPUS_MODEL` | Hard reasoning, architecture |
| Haiku | `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Fast background tasks |
| Subagent | `CLAUDE_CODE_SUBAGENT_MODEL` | Background agents |

Optional tiers fall back to sonnet if not set.

## Development

```bash
make build      # → bin/clavis
make install    # → ~/.local/bin/clavis
make test       # go test ./...
make lint       # go vet ./...
```

## License

[MIT](LICENSE) © 2026 Long Nguyen
