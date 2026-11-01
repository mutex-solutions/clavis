---
name: clavis
description: Use when launching Claude Code on a non-Anthropic provider, when a clavis profile looks wrong ("no models", "wrong model", "can't switch models"), or when routing subagents to models served by a gateway such as ai.dinhlongviolin1.com.
---

# clavis — provider routing for Claude Code

Clavis writes a profile (base URL + model tiers + key), then launches `claude` with
the matching environment. It never edits `~/.claude/settings.json`; everything is
either an env var on the child process or a `--settings` file it generates.

Profiles live in `~/.clavis/profiles/<name>/`:
`profile.json` (no secrets), `.env` (0600, holds `CLAVIS_KEY`), `state.json` at the
root holds `last_used`. `CLAVIS_HOME` overrides the root — use it for throwaway
tests instead of touching the real profiles.

## Read a profile before answering anything about it

```sh
clavis list                        # profiles + which is last used
clavis get <name> --json           # profile + resolved tiers + key_present
clavis test <name> --json          # config, key, claude on PATH, every tier pinged
```

`clavis get --json` resolves tier fallbacks (`opus`, `haiku`, `subagent`, `fable`
all default to sonnet), so read it rather than re-deriving from `sonnet_model`.
`clavis test` is the only honest check that a profile works; a profile that lists
models but fails its ping is not usable.

## Launch

```sh
clavis <name>                      # interactive
clavis <name> -- -p "…"            # everything after -- goes to claude
eval "$(clavis env <name> --reveal-key)"   # bind-mode profiles only
```

- `model_mode: pick` profiles bind nothing: every launch asks for the main
  model and optionally a model per tier (Fable, Sonnet, Haiku, Subagent), plus a
  context-window override. Nothing is saved, so the next launch asks again.
  Opus is the session model; Fable sits above it, and every unset tier falls
  back to the primary.
- A setup-token profile may bind no model at all: the wizard offers "Use
  Claude's default model", and `clavis add --provider claude` without `--model`
  does the same.
- `clavis env` **refuses pick-mode profiles** on purpose — there is no fixed
  model to export. Bind one with `clavis edit`, or launch through clavis.
- `clavis <name> --settings <file>` keeps the caller's settings: clavis drops its
  own generated picker file rather than stacking a second `--settings`.

## Direct Claude (setup token)

`claude setup-token` mints a long-lived token for a Claude subscription. It goes
in a profile whose `auth_mode` is `oauth`:

```sh
clavis add --provider claude --setup-token --alias personal
```

That runs the official `claude setup-token` browser flow on a pty, captures the
token it prints, and saves the profile — one step. It needs a terminal, so it
cannot be used from an agent's non-TTY shell; there, mint the token yourself and
use `--key-stdin`. Windows has no automatic capture.

At launch that profile exports `CLAUDE_CODE_OAUTH_TOKEN` and nothing else — no
`ANTHROPIC_BASE_URL`, no `ANTHROPIC_AUTH_TOKEN`, and the inherited copies are
stripped so a shell already pointed at a gateway cannot capture the session.
With no model bound, Claude's own default applies.

- Validation lists models; it does not ping. A messages ping spends quota, and a
  busy account answers 429 for a token that is fine.
- Multiple accounts = multiple profiles, each with its own token in its `.env`.
  Sessions run side by side (the credential is per-process). Quota is per
  account; two profiles on one account share the same limits.
- Never point one of these at a gateway. Replaying a subscription's OAuth
  through a proxy is the pattern Anthropic bans, and is why the cliproxy
  gateway excludes Claude.

## The `/model` picker

At launch, clavis fetches the provider's `/v1/models`, writes
`~/.clavis/profiles/<name>/claude-settings.json` with a `modelPicker.options`
array, and passes it as `--settings`. Claude Code reads that **in addition to**
the user's settings.

Why it exists: Claude Code's own gateway discovery keeps only ids containing
`claude` or `anthropic`, which no third-party gateway satisfies. Turning on
`CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` therefore finds nothing here.
`ANTHROPIC_CUSTOM_MODEL_OPTION` adds exactly one id; `modelPicker` lists many.

Without a relaunch, `/model <id>` typed directly still works if the gateway
serves the id — Claude Code does not pre-validate `--model`. The
`[claude-code:unrecognized_model]` line is cosmetic.

## Subagents and other models

**Same endpoint, different model — supported.** Agent frontmatter takes a full
id:

```yaml
---
name: reviewer
description: …
model: gemini-3.8-flash-high
---
```

Frontmatter beats `CLAUDE_CODE_SUBAGENT_MODEL` (since v2.1.251); setting
`CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` would flatten every agent back to one
model. The Agent tool's own `model` parameter accepts only aliases
(`sonnet|opus|haiku|fable`), so per-agent ids must come from a definition file.

**Different endpoint — impossible in one process.** `ANTHROPIC_BASE_URL` is
process-wide and there is no per-agent env, base URL, or credential field.
Unknown frontmatter keys are ignored silently, so an `env:` block does nothing.
Two endpoints means two `claude` processes:

```sh
clavis <profile> -- -p "task"        # child process on the gateway
```

That child shares `~/.claude` (MCP servers, agents, memory) but has its own
context; only stdout comes back. For a native-Claude parent with a gateway
child, strip the inherited clavis environment first:

```sh
env -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL \
    -u ANTHROPIC_DEFAULT_SONNET_MODEL -u ANTHROPIC_DEFAULT_OPUS_MODEL \
    -u ANTHROPIC_DEFAULT_HAIKU_MODEL -u ANTHROPIC_DEFAULT_FABLE_MODEL \
    -u CLAUDE_CODE_SUBAGENT_MODEL -u CLAUDE_CODE_MAX_CONTEXT_TOKENS \
    -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC \
    -u CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS \
    claude
```

Missing one of the tier vars leaves subagents on the old provider even after
the base URL is gone — strip the whole set.

**Never put a Claude subscription behind a gateway.** Replaying its OAuth
through a proxy is the pattern Anthropic bans; the cliproxy runbook rejects it
for that reason. Direct `claude /login` per machine, or a pay-as-you-go API
key, are the legitimate routes.

## Traps

- A model id must be one the profile's base URL actually serves. `clavis test`
  pings tiers for exactly this reason; `deepseek-flash` on a Kimi profile fails
  at request time, not at launch.
- `--settings` is ignored mid-session: picker changes need a relaunch.
- Writing files: `claude-settings.json` is generated on every launch; do not
  hand-edit it, edit the catalogue or the code.
- Doc drift is a bug. Update `README.md`, `SPEC.md`, and this skill together.
