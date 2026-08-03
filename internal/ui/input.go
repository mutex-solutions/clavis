package ui

import (
	"errors"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// errTerminalClosed is returned by terminalInput.Read when the terminal goes
// away (EOF). bubbletea surfaces it as a program error so Run() returns.
var errTerminalClosed = errors.New("terminal input closed")

// terminalInput wraps os.Stdin so that EOF is surfaced as an error instead of
// being silently swallowed by bubbletea. bubbletea's readLoop deliberately
// ignores io.EOF from its input reader, which leaves the program running
// forever with no input — orphaning the process when its terminal is closed.
// Converting EOF to a real error makes bubbletea's Run() return so clavis can
// exit.
//
// The type embeds *os.File so it still satisfies charmbracelet/x/term's File
// interface (Fd()): bubbletea sets raw mode on it and cancelreader uses its fd
// for epoll-based cancellation, exactly as if it were the raw os.Stdin.
type terminalInput struct {
	*os.File
}

func (t terminalInput) Read(p []byte) (int, error) {
	n, err := t.File.Read(p)
	if errors.Is(err, io.EOF) {
		return n, errTerminalClosed
	}
	return n, err
}

// stdinTTY returns a bubbletea program option that reads from os.Stdin while
// treating EOF as a terminal failure, so a closed terminal exits the program
// instead of leaving it orphaned.
func stdinTTY() tea.ProgramOption {
	return tea.WithInput(terminalInput{os.Stdin})
}
