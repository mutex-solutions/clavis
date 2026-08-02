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

## Optional: Homebrew + Scoop

The release also publishes a Homebrew cask and a Scoop manifest **if** the
corresponding tokens are set — otherwise GoReleaser skips them and the release
still succeeds (the `skip_upload` guards handle this).

To turn them on:

### Homebrew (`brew install dinhlongviolin1/tap/clavis`)

1. Create a public repo named **`homebrew-tap`** under your account (the
   `homebrew-` prefix is required by Homebrew).
2. Create a fine-grained PAT with **Contents: read/write** on that repo.
3. Add it to the clavis repo as a secret named **`HOMEBREW_TAP_TOKEN`**
   (Settings → Secrets and variables → Actions → New repository secret).
4. Cut the next release. GoReleaser pushes the cask to `homebrew-tap`.

Users then run:

```bash
brew install dinhlongviolin1/tap/clavis
```

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
