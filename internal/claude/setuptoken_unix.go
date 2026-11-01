//go:build !windows

package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/creack/pty"
)

// ErrSetupTokenUnreadable reports that `claude setup-token` finished but its
// output did not contain a token clavis could read. The caller falls back to
// asking for a pasted token.
var ErrSetupTokenUnreadable = errors.New("could not read the token from claude setup-token output")

// ansiRE matches the cursor and colour sequences the TUI emits around text.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]|\x1b\\][^\x07]*\x07|\x1b[()][A-Z0-9]")

// setupTokenRE matches a Claude setup token. The prefix is what
// `claude setup-token` prints today; the run is bounded because a longer tail
// means the scrape swallowed neighbouring text.
var setupTokenRE = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9_-]{40,160}`)

// RunSetupToken drives `claude setup-token` on the user's terminal and returns
// the token it prints.
//
// The command owns the browser flow and the consent screen; clavis only gives
// it a pty so the interactive UI works, widens the terminal so a 108-character
// token cannot wrap, and scrapes the result. The child runs without the shell's
// credential variables, so it does not warn that authentication is already
// configured.
func RunSetupToken(claudePath string) (string, error) {
	return runSetupToken(claudePath, os.Stdout)
}

func runSetupToken(claudePath string, out io.Writer) (string, error) {
	cmd := exec.Command(claudePath, "setup-token")
	cmd.Env = cleanEnv()

	// 400 columns: wide enough that the token prints on one line, which is what
	// makes the scrape below exact rather than a guess across wrapped chunks.
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 50, Cols: 400})
	if err != nil {
		return "", fmt.Errorf("start claude setup-token: %w", err)
	}

	var buf bytes.Buffer
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		b := make([]byte, 4096)
		for {
			n, rerr := ptmx.Read(b)
			if n > 0 {
				_, _ = out.Write(b[:n])
				mu.Lock()
				buf.Write(b[:n])
				mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()

	// Keys and pasted codes go to the child; the reader above forwards its
	// replies (cursor-position queries) back through the same terminal.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()

	waitErr := cmd.Wait()
	_ = ptmx.Close() // unblocks the reader if the child left it open
	<-done

	mu.Lock()
	rendered := ansiRE.ReplaceAllString(buf.String(), "")
	mu.Unlock()

	if token := extractSetupToken(rendered); token != "" {
		return token, nil
	}
	if waitErr != nil {
		return "", fmt.Errorf("claude setup-token exited: %w", waitErr)
	}
	return "", ErrSetupTokenUnreadable
}

// extractSetupToken returns the longest token-shaped run in the output, or "".
func extractSetupToken(out string) string {
	best := ""
	for _, t := range setupTokenRE.FindAllString(out, -1) {
		if len(t) > len(best) {
			best = t
		}
	}
	return best
}
