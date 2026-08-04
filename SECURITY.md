# Security Policy

`clavis` is a provider/credential manager for Claude Code: it stores per-profile API keys
on disk and injects them into the agent it launches. Please treat it as security-sensitive.

## How keys are stored
- API keys live **only** in per-profile files — `~/.clavis/profiles/<name>/.env`, written
  with `0600` permissions (owner read/write only). They are never stored in this repository.
- `clavis env <name>` **redacts** the key by default and refuses to print it straight to a
  terminal; `--reveal-key` is required to emit the real value (intended for
  `eval "$(clavis env <name> --reveal-key)"`).
- On launch, clavis exports the key as an environment variable (e.g. `ANTHROPIC_AUTH_TOKEN`)
  into the Claude Code process it starts.

## Threat model / operator notes
- **Any process clavis launches inherits the key via its environment.** Don't launch a
  profile against code, tools, or an agent you don't trust with that credential.
- Keys are stored at rest as plaintext `.env` files (mode `0600`). Protect your home
  directory and disk accordingly, and prefer per-provider keys scoped to least privilege.
- Never commit a profile's `.env`, and never paste `--reveal-key` output anywhere shared.

## Reporting a vulnerability
Please report privately — **do not open a public issue**:
- Preferred: a **GitHub private security advisory** (repo → **Security → Report a vulnerability**).
- Or contact the maintainer via the [`dinhlongviolin1`](https://github.com/dinhlongviolin1) GitHub profile.

We'll acknowledge within a few days. As a pre-1.0 project there are no formal SLAs, but
security reports are prioritized over features.

## Secret hygiene in this repo
Secret scanning is enforced by [gitleaks](https://github.com/gitleaks/gitleaks) in CI
(full git history) and as a pre-commit hook. See [`.gitleaks.toml`](.gitleaks.toml) and
[`.gitignore`](.gitignore); never commit tokens, keys, or `.env` files.
