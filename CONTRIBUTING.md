# Contributing to clavis

Thanks for your interest! clavis is a small, focused Go CLI and contributions
are welcome.

## Development setup

Requires **Go 1.23+**.

```bash
git clone https://github.com/dinhlongviolin1/clavis.git
cd clavis
make build      # → bin/clavis
make test       # go test -race ./...
make lint       # go vet ./...
make fmt        # gofmt -w .
```

## Before you open a PR

Please make sure:

- `gofmt -l .` reports nothing (run `make fmt`).
- `go vet ./...` passes (`make lint`).
- `go test -race ./...` passes (`make test`).
- New behavior has a test. The wizard/TUI is hard to unit-test end to end, but
  the pure logic (item ordering, env building, name validation, exit codes)
  should be covered.

CI runs all of the above plus a cross-compile matrix (linux/darwin/windows ×
amd64/arm64) and a `gitleaks` secret scan on every push and PR.

## Project layout

```
cmd/clavis/          # CLI entrypoint, subcommand routing, headless add, update
internal/profile/    # Profile model, storage (atomic writes), validation
internal/provider/   # Provider catalog, live model fetch, key ping
internal/claude/     # Env building + spawning claude (platform-specific files)
internal/ui/         # huh-based wizard, custom model selector, spinner, styles
```

## Design principles

- **Launcher, not a proxy.** clavis sets env vars and execs `claude`; it never
  sits in the request path.
- **Never leak secrets.** Keys live only in per-profile `.env` files (0600) and
  are stripped from the child env if inherited. Nothing secret goes in
  `profile.json` or logs.
- **Share the user's `~/.claude` by default.** Isolation is opt-in.
- **Agent-friendly.** Every interactive flow has a headless equivalent with
  `--json` output and stable exit codes.
- **Cross-platform.** Keep platform-specific syscalls behind build-tagged files
  (`launch_linux.go`, `launch_darwin.go`, `launch_windows.go`).

## Adding a provider

Providers live in `internal/provider/providers.json` (embedded via `go:embed`).
Add an entry with `id`, `displayName`, `baseUrl`, optional `modelsUrl` (if the
model-list endpoint differs from the messages endpoint), and `auth`. Prefer the
**custom** provider for region-specific endpoints rather than hard-coding a URL
that only works in one region.

## Commit messages

Conventional-ish prefixes are used for the changelog (`feat:`, `fix:`, `docs:`,
`test:`, `ci:`, `chore:`). `docs`/`test`/`ci`/`chore` are excluded from release
notes.

## License

By contributing you agree your contributions are licensed under the
[Apache License 2.0](LICENSE).
