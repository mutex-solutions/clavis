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

## Homebrew (`brew install mutex-solutions/tap/clavis`)

The tap repo **`mutex-solutions/homebrew-tap`** already exists and carries a
formula that works on macOS **and** Linux (amd64 + arm64). Users install with:

```bash
brew install mutex-solutions/tap/clavis
```

### Auto-update on release

`.github/workflows/release.yml` regenerates the formula from the release
checksums (`scripts/gen-formula.sh`) and pushes it to the tap. It authenticates
with a **write-enabled deploy key** on the tap repo, held in the
`HOMEBREW_TAP_DEPLOY_KEY` secret. If that secret is unset the step self-skips
and the release still succeeds; you would just update the tap manually (below).

Setup, recorded for rebuilds:

1. Generate a keypair, and add the public half to `mutex-solutions/homebrew-tap`
   as a deploy key **with write access**.
2. Store the private half as the clavis secret **`HOMEBREW_TAP_DEPLOY_KEY`**.
3. Deploy keys must be enabled for the org. `mutex-solutions` is new, and GitHub
   disables them for new organisations by default; set the org's
   `deploy_keys_enabled_for_repositories` to true.

A deploy key rather than a PAT, because it is scoped to the single repository it
is installed on and it does not expire.

> **Post-mortem: why this is not a PAT any more.** Until 2026-10-13 the step used
> `HOMEBREW_TAP_TOKEN`, a fine-grained PAT. Two things went wrong at once when
> clavis moved into the org:
>
> 1. **Repository secrets survive a transfer.** The old PAT came across, so the
>    step did not self-skip. A present-but-mis-scoped secret *fails* the release,
>    where an absent one merely skips it. `v0.3.0` published its binaries and
>    then went red on `git push`: `403 … Permission to
>    mutex-solutions/homebrew-tap.git denied to <you>`.
> 2. A PAT is scoped to a **user**. After the move the tap belongs to an **org**,
>    so a token minted before it cannot write after it.
>
> The deploy key removes both failure modes.

### Update the tap by hand

```bash
sh scripts/gen-formula.sh 0.2.1 > Formula/clavis.rb   # from within a tap checkout
# commit + push to mutex-solutions/homebrew-tap
```

## Validate the config locally

Before tagging, you can sanity-check GoReleaser without publishing:

```bash
goreleaser check                                   # validate .goreleaser.yml
goreleaser build --snapshot --clean --single-target # test-build, no publish
```
