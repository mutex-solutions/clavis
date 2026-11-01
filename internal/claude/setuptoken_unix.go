//go:build !windows

package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// ErrSetupTokenUnreadable reports that `claude setup-token` finished but its
// output did not contain a token clavis could read. The caller falls back to
// asking for a pasted token.
var ErrSetupTokenUnreadable = errors.New("could not read the token from claude setup-token output")

// ansiRE matches the cursor and colour sequences the TUI emits around text.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]|\x1b\\][^\x07]*\x07|\x1b[()][A-Z0-9]")

// setupTokenPrefix and setupTokenLen describe the token `claude setup-token`
// prints (measured: 108 characters, sk-ant-oat01- followed by 95 more). The
// length is what makes a wrapped token recoverable: the terminal renderer breaks
// a long line at the window width, so the scrape cannot rely on finding one
// contiguous run.
const (
	setupTokenPrefix = "sk-ant-oat01-"
	setupTokenLen    = 108
)

// setupTokenRE matches a token that arrived on one line.
var setupTokenRE = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9_-]{40,160}`)

// RunSetupToken drives `claude setup-token` on the user's terminal and returns
// the token it prints.
//
// The command owns the browser flow and the consent screen; clavis only gives it
// a pty sized to the real terminal, puts the local terminal into raw mode so the
// TUI receives keystrokes as it does standalone, and scrapes the result. The
// child runs without the shell's credential variables, so it does not warn that
// authentication is already configured.
func RunSetupToken(claudePath string) (string, error) {
	return runSetupToken(claudePath, os.Stdout)
}

func runSetupToken(claudePath string, out io.Writer) (string, error) {
	cmd := exec.Command(claudePath, "setup-token")
	cmd.Env = cleanEnv()

	// Start at the user's real size. A wider pty looks fine to the child and
	// produces a UI the actual terminal wraps into nonsense; the token wrap is
	// handled in extractSetupToken instead of avoided here.
	size := &pty.Winsize{Rows: 24, Cols: 80}
	if cols, rows, serr := term.GetSize(int(os.Stdout.Fd())); serr == nil {
		size = &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	}
	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return "", fmt.Errorf("start claude setup-token: %w", err)
	}

	// Follow terminal resizes while the flow is on screen.
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	defer signal.Stop(resize)
	go func() {
		for range resize {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()

	// The child's TUI expects raw keystrokes. Without this the local line
	// discipline buffers input until Enter, and terminal replies (focus events,
	// cursor queries) are echoed into the form instead of being understood.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		if state, rerr := term.MakeRaw(int(os.Stdin.Fd())); rerr == nil {
			defer func() { _ = term.Restore(int(os.Stdin.Fd()), state) }()
		}
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

// extractSetupToken returns the token in the output, or "".
//
// The fast path is one contiguous run. At the user's real terminal width the
// token wraps, and the renderer breaks it with a newline plus padding, so the
// fallback rebuilds it from the token charset — ignoring whitespace and the
// escape sequences already stripped — up to the known length.
func extractSetupToken(out string) string {
	best := ""
	for _, t := range setupTokenRE.FindAllString(out, -1) {
		if len(t) > len(best) {
			best = t
		}
	}
	if len(best) >= setupTokenLen {
		return best
	}

	i := strings.Index(out, setupTokenPrefix)
	if i < 0 {
		return best
	}
	var b strings.Builder
	for _, r := range out[i:] {
		if b.Len() >= setupTokenLen {
			break
		}
		if isTokenRune(r) {
			b.WriteRune(r)
		}
	}
	if b.Len() >= setupTokenLen {
		return b.String()
	}
	return best
}

func isTokenRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		return true
	case r == '-', r == '_':
		return true
	}
	return false
}
