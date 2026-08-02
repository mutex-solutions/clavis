# Releasing clavis

Releases are fully automated by GoReleaser on tag push.

## Cut a release

```bash
git tag -a v0.2.0 -m "clavis v0.2.0"
git push origin v0.2.0
```

That triggers `.github/workflows/release.yml`, which runs GoReleaser and:

- cross-compiles 6 binaries (linux/darwin/windows × amd64/arm64),
- packs them (`clavis_<os>_<arch>.tar.gz`, `.zip` for Windows) + `checksums.txt`,
- creates the GitHub Release and attaches everything.

`install.sh`, `install.ps1`, and `clavis update` all read from this release. No
manual build or upload.

## Versioning

The binary's version comes from the git tag via `-ldflags -X main.Version`
(Go 1.26 requires the `main.` prefix, not the full import path). A build with no
tag reports `dev`. Verify after tagging:

```bash
clavis version   # should print the tag, e.g. "clavis 0.2.0"
```

## Homebrew (`brew install dinhlongviolin1/tap/clavis`)

The tap repo **`dinhlongviolin1/homebrew-tap`** already exists and carries a
formula that works on macOS **and** Linux (amd64 + arm64). Users install with:

```bash
brew install dinhlongviolin1/tap/clavis
```

### Auto-update on release

`.github/workflows/release.yml` regenerates the formula from the release
checksums (`scripts/gen-formula.sh`) and pushes it to the tap — but only when a
`HOMEBREW_TAP_TOKEN` secret is set. Without it, the step is skipped and the
release still succeeds; you'd just update the tap manually (below).

To enable auto-update:

1. Create a fine-grained PAT with **Contents: read/write** on
   `dinhlongviolin1/homebrew-tap`.
2. Add it to the clavis repo as the secret **`HOMEBREW_TAP_TOKEN`**
   (Settings → Secrets and variables → Actions).
3. Future releases update the tap automatically.

### Update the tap by hand

```bash
sh scripts/gen-formula.sh 0.2.1 > Formula/clavis.rb   # from within a tap checkout
# commit + push to dinhlongviolin1/homebrew-tap
```

## Optional: Scoop (Windows)

GoReleaser publishes a Scoop manifest **if** `SCOOP_BUCKET_TOKEN` is set;
otherwise it's skipped and the release still succeeds.

### Scoop (Windows: `scoop install clavis`)

1. Create a public repo named **`scoop-bucket`**.
2. Create a PAT with Contents: read/write on it.
3. Add it as the secret **`SCOOP_BUCKET_TOKEN`**.
4. Cut the next release.

Users then run:

```powershell
scoop bucket add dinhlongviolin1 https://github.com/dinhlongviolin1/scoop-bucket
scoop install clavis
```

## Validate the config locally

Before tagging, you can sanity-check GoReleaser without publishing:

```bash
goreleaser check                                   # validate .goreleaser.yml
goreleaser build --snapshot --clean --single-target # test-build, no publish
```
