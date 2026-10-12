package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mutex-solutions/clavis/internal/ui"
	"golang.org/x/mod/semver"
)

// The repo self-updates pull from. Overridable at build time via ldflags if a
// fork or mirror is used.
var UpdateRepo = "mutex-solutions/clavis"

// Result of comparing the running build to a release tag.
const (
	cmpDev   = iota // current build isn't a real release version
	cmpOlder        // current < latest (an update is available)
	cmpEqual        // current == latest
	cmpNewer        // current > latest (ahead of the release; don't downgrade)
)

// compareVersions orders the running version against a release tag using
// semver. `current` is the version without a leading "v"; `tag` is the raw
// release tag (with or without "v"). Anything that isn't valid semver — "dev",
// "", a git SHA like "5ddc0db-dirty" — is treated as cmpDev.
func compareVersions(current, tag string) int {
	cur := "v" + strings.TrimPrefix(current, "v")
	rel := "v" + strings.TrimPrefix(tag, "v")
	if !semver.IsValid(cur) || !semver.IsValid(rel) {
		return cmpDev
	}
	switch semver.Compare(cur, rel) {
	case -1:
		return cmpOlder
	case 1:
		return cmpNewer
	default:
		return cmpEqual
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// runUpdate implements `clavis update [--check]`: self-update from the latest
// GitHub release matching this OS/arch.
func runUpdate(args []string) {
	checkOnly := false
	for _, a := range args {
		if a == "--check" {
			checkOnly = true
		} else if a == "-h" || a == "--help" {
			fmt.Println(`clavis update — update clavis to the latest release

Usage:
  clavis update            Download and install the latest version
  clavis update --check    Report whether an update is available (no install)`)
			return
		}
	}

	fmt.Println(ui.DimStyle.Render("  Checking for updates…"))
	rel, err := latestRelease()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Could not check for updates: %v", err)))
		os.Exit(1)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	// Compare using semver ordering, not string equality, so we never offer a
	// downgrade and correctly recognise "already up to date".
	switch cmp := compareVersions(current, rel.TagName); {
	case cmp == cmpDev:
		// Non-release build (go install, local build): can't reason about
		// ordering, so just report the latest and let the user decide.
		fmt.Println(ui.Warn("Running a dev build — the latest release is " + latest + "."))
	case cmp == cmpNewer:
		fmt.Println(ui.Success(fmt.Sprintf("You're ahead of the latest release (local %s > %s)", current, latest)))
		return
	case cmp == cmpEqual:
		fmt.Println(ui.Success(fmt.Sprintf("Already on the latest version (%s)", current)))
		return
	}

	fmt.Printf("  %s → %s\n", ui.DimStyle.Render(current), ui.CommandStyle.Render(latest))
	if checkOnly {
		fmt.Println(ui.NextStep("Update with:", "clavis update"))
		fmt.Println(ui.DimStyle.Render("  Release notes: " + rel.HTMLURL))
		return
	}

	// Find the asset for this platform. goreleaser names archives
	// clavis_<os>_<arch>.tar.gz (lowercase; Windows uses .zip, handled below).
	wantOS := runtime.GOOS
	wantArch := runtime.GOARCH
	ext := ".tar.gz"
	if wantOS == "windows" {
		ext = ".zip"
	}
	suffix := fmt.Sprintf("_%s_%s%s", wantOS, wantArch, ext)

	var assetURL string
	for _, a := range rel.Assets {
		// Match case-insensitively on the OS/arch/ext suffix.
		if strings.HasSuffix(strings.ToLower(a.Name), strings.ToLower(suffix)) {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("No release asset for %s/%s in %s", wantOS, wantArch, latest)))
		fmt.Fprintln(os.Stderr, ui.DimStyle.Render("  Download manually: "+rel.HTMLURL))
		os.Exit(1)
	}

	if ext == ".zip" {
		// Self-replace of a running .exe is unreliable on Windows; guide instead.
		fmt.Println(ui.Warn("Automatic update isn't supported on Windows."))
		fmt.Println(ui.DimStyle.Render("  Download the new build: " + assetURL))
		os.Exit(1)
	}

	fmt.Println(ui.DimStyle.Render("  Downloading " + latest + "…"))
	newBin, err := downloadBinary(assetURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Download failed: %v", err)))
		os.Exit(1)
	}

	if err := replaceSelf(newBin); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("Install failed: %v", err)))
		os.Exit(1)
	}

	fmt.Println(ui.Success(fmt.Sprintf("Updated to %s", latest)))

	// Refresh any already-installed shell completions so new subcommands show
	// up. Best-effort: does nothing if completions were never installed.
	if shells := refreshInstalledCompletions(); len(shells) > 0 {
		fmt.Println(ui.DimStyle.Render("  Refreshed shell completions: " + strings.Join(shells, ", ")))
		// A zsh completion whose content changed needs its cache cleared.
		for _, s := range shells {
			if s == "zsh" {
				fmt.Println(ui.DimStyle.Render("  For zsh, run: rm -f ~/.zcompdump* && exec zsh"))
				break
			}
		}
	}
}

// latestRelease fetches the newest published (non-prerelease) release.
func latestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", UpdateRepo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "clavis-updater")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases published yet")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("malformed release data")
	}
	return &rel, nil
}

// downloadBinary fetches a .tar.gz release archive and extracts the clavis
// binary into a temp file next to the current executable. Returns its path.
func downloadBinary(url string) (string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "clavis-updater")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d fetching archive", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(self)

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "clavis" {
			continue
		}
		tmp, err := os.CreateTemp(dir, ".clavis-update-*")
		if err != nil {
			// Fall back to the system temp dir if the install dir isn't writable
			// (the rename step will surface a clear permission error later).
			tmp, err = os.CreateTemp("", ".clavis-update-*")
			if err != nil {
				return "", err
			}
		}
		if _, err := io.Copy(tmp, io.LimitReader(tr, 200<<20)); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		tmp.Close()
		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		return tmp.Name(), nil
	}
	return "", fmt.Errorf("clavis binary not found in archive")
}

// replaceSelf atomically swaps the running executable with newBin.
func replaceSelf(newBin string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return err
	}

	// A running binary can't be written in place, but it CAN be renamed on
	// Unix; move the new file over it. If they're on different filesystems the
	// rename fails, so fall back to a copy.
	if err := os.Rename(newBin, self); err != nil {
		if cpErr := copyFile(newBin, self); cpErr != nil {
			return fmt.Errorf("could not replace %s: %v (writable? try sudo)", self, cpErr)
		}
		os.Remove(newBin)
	}
	return os.Chmod(self, 0o755)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Write to a temp then rename for atomicity.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".clavis-swap-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
