package ui

import (
	"os"

	"golang.org/x/term"
)

// IsInteractive reports whether clavis is attached to a real terminal on both
// stdin and stdout. Interactive flows (the wizard, the profile selector,
// confirm prompts) require this — under an agent runner like Hermes/OpenClaw
// there is no TTY, and a bubbletea/huh form would otherwise render escape codes
// into the agent's log and exit as if it had succeeded.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// StdoutIsTTY reports whether stdout specifically is a terminal. Used to decide
// whether human-facing chrome (the launch banner) is safe to print, or whether
// it would corrupt a data stream an agent is parsing.
func StdoutIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
