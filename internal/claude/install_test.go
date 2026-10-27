package claude

import (
	"strings"
	"testing"
)

func TestInstallCommandPerPlatform(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		name, args := InstallCommand(goos)
		if name != "sh" {
			t.Errorf("%s: program = %q, want sh", goos, name)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "https://claude.ai/install.sh") {
			t.Errorf("%s: args = %q, want the install.sh URL", goos, joined)
		}
	}

	name, args := InstallCommand("windows")
	if name != "powershell" {
		t.Errorf("windows: program = %q, want powershell", name)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "https://claude.ai/install.ps1") {
		t.Errorf("windows: args = %q, want the install.ps1 URL", joined)
	}
}

func TestInstallHintMatchesPlatform(t *testing.T) {
	hint := InstallHint()
	if hint == "" {
		t.Fatal("InstallHint() is empty")
	}
	if !strings.Contains(hint, "install.") {
		t.Errorf("InstallHint() = %q, want an installer URL", hint)
	}
}
