package claude

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// The native installer is per-user on every platform: it needs no sudo and no
// Administrator, and puts the binary in ~/.local/bin (a symlink into
// ~/.local/share/claude/versions). npm is deliberately not used as the default
// here: the docs warn against sudo npm installs, and the native path does not
// create a root-owned prefix in the first place.
const (
	installURLUnix = "https://claude.ai/install.sh"
	installURLWin  = "https://claude.ai/install.ps1"
)

// InstallCommand returns the program and arguments that install Claude Code on
// goos. It is split out from Install so the command can be shown, tested, and
// printed on a host where nothing may be executed.
func InstallCommand(goos string) (string, []string) {
	if goos == "windows" {
		return "powershell", []string{
			"-NoProfile", "-ExecutionPolicy", "Bypass",
			"-Command", "irm " + installURLWin + " | iex",
		}
	}
	return "sh", []string{"-c", "curl -fsSL " + installURLUnix + " | bash"}
}

// InstallHint is the command to show a human, in the shell they are most
// likely to be using.
func InstallHint() string {
	if runtime.GOOS == "windows" {
		return `powershell -Command "irm ` + installURLWin + ` | iex"`
	}
	return "curl -fsSL " + installURLUnix + " | bash"
}

// FindClaude returns the path to the claude binary. PATH is checked first, then
// the native installer's per-user directory: an install that completed in this
// session does not update the current process's PATH, so the fallback is what
// makes "install now, launch immediately" work.
func FindClaude() (string, error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}
	for _, path := range nativePaths() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("Claude Code is not installed or not on PATH")
}

func nativePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(home, ".local", "bin", "claude.exe"),
			filepath.Join(home, ".claude", "local", "claude.exe"),
		}
	}
	return []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
	}
}

// Install runs the native installer, streaming its output to the terminal.
func Install() error {
	name, args := InstallCommand(runtime.GOOS)
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// OfferInstall is what Launch calls when claude is missing. In a terminal it
// asks, installs, and returns the new path; without a terminal it refuses to
// block and returns the command to run instead.
func OfferInstall() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("Claude Code is not installed — install with: %s", InstallHint())
	}

	fmt.Printf("Claude Code is not installed.\n\n"+
		"Install it now with the official installer?\n\n  %s\n\n"+
		"This installs per-user, no sudo or Administrator. [Y/n] ", InstallHint())

	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y", "yes":
	default:
		return "", fmt.Errorf("Claude Code is not installed — install with: %s", InstallHint())
	}

	fmt.Println()
	if err := Install(); err != nil {
		return "", fmt.Errorf("installer failed: %w", err)
	}
	path, err := FindClaude()
	if err != nil {
		return "", fmt.Errorf("install finished, but claude is still not on PATH — open a new terminal, then run clavis again")
	}
	fmt.Printf("Installed: %s\n\n", path)
	return path, nil
}
