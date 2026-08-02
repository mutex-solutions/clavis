package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionSpecFor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, sh := range supportedShells() {
		spec, err := completionSpecFor(sh)
		if err != nil {
			t.Fatalf("completionSpecFor(%q) error: %v", sh, err)
		}
		if spec.script == "" {
			t.Errorf("%s: empty script", sh)
		}
		if !strings.HasPrefix(spec.path, home) {
			t.Errorf("%s: path %q not under home %q", sh, spec.path, home)
		}
	}

	if _, err := completionSpecFor("powershell"); err == nil {
		t.Error("expected error for unsupported shell")
	}
}

func TestRefreshOnlyTouchesInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Install just zsh with stale content.
	zshSpec, _ := completionSpecFor("zsh")
	if err := os.MkdirAll(filepath.Dir(zshSpec.path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zshSpec.path, []byte("STALE"), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed := refreshInstalledCompletions()

	// Only zsh was installed, so only zsh should be refreshed.
	if len(refreshed) != 1 || refreshed[0] != "zsh" {
		t.Fatalf("refreshed = %v, want [zsh]", refreshed)
	}

	// The stale content must be replaced with the real script.
	got, _ := os.ReadFile(zshSpec.path)
	if string(got) == "STALE" {
		t.Error("zsh completion was not refreshed")
	}
	if !strings.Contains(string(got), "update") {
		t.Error("refreshed zsh completion missing the 'update' subcommand")
	}

	// bash/fish were never installed → their files must NOT be created.
	bashSpec, _ := completionSpecFor("bash")
	if _, err := os.Stat(bashSpec.path); err == nil {
		t.Error("refresh created a bash completion that was never installed")
	}
}

func TestRefreshNoInstalledIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if refreshed := refreshInstalledCompletions(); len(refreshed) != 0 {
		t.Errorf("expected no refresh on a clean home, got %v", refreshed)
	}
}

func TestWriteCompletionRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	spec, _ := completionSpecFor("fish")
	if err := writeCompletion(spec); err != nil {
		t.Fatalf("writeCompletion error: %v", err)
	}
	got, err := os.ReadFile(spec.path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != fishCompletion {
		t.Error("written fish completion does not match source")
	}
}
