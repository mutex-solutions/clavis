//go:build !windows

package claude

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A token of the real shape: 108 characters.
var sampleToken = "sk-ant-oat01-" + strings.Repeat("A1b-", 23) + "xyz"

func TestExtractSetupTokenPlain(t *testing.T) {
	out := "Your OAuth token:\n" + sampleToken + "\n\nStore it somewhere safe."
	if got := extractSetupToken(out); got != sampleToken {
		t.Errorf("got %q", got)
	}
}

func TestExtractSetupTokenStripsANSI(t *testing.T) {
	// The TUI pads and repositions text; the token itself is one run.
	out := "\x1b[38;2;255;193;7mToken\x1b[9G" + sampleToken + "\x1b[39m\r\n"
	if got := extractSetupToken(out); got != sampleToken {
		t.Errorf("got %q", got)
	}
}

func TestExtractSetupTokenLongestWins(t *testing.T) {
	short := sampleToken[:60]
	out := short + "\n" + sampleToken
	if got := extractSetupToken(out); got != sampleToken {
		t.Errorf("got %q, want the longer run", got)
	}
}

func TestExtractSetupTokenWrapped(t *testing.T) {
	// At a normal terminal width the renderer breaks the token across lines.
	wrapped := sampleToken[:70] + "\r\n  " + sampleToken[70:]
	if got := extractSetupToken(wrapped); got != sampleToken {
		t.Errorf("got %q, want the reassembled token", got)
	}
}

func TestRunSetupTokenCapturesThroughPTY(t *testing.T) {
	// A stand-in for claude: prints the token the way the TUI does, on a pty.
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-claude")
	script := "#!/bin/sh\nprintf '\\033[32mYour OAuth token:\\033[0m " + sampleToken + "\\n'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := runSetupToken(path, io.Discard)
	if err != nil {
		t.Fatalf("runSetupToken: %v", err)
	}
	if got != sampleToken {
		t.Errorf("got %q, want %q", got, sampleToken)
	}
}

func TestExtractSetupTokenAbsent(t *testing.T) {
	if got := extractSetupToken("no token here\n" + strings.Repeat("x", 200)); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
