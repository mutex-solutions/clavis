package ui

import (
	"bytes"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// openPty returns a master/slave pty pair, like a real terminal.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	if err := unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		m.Close()
		t.Fatalf("unlockpt: %v", err)
	}
	n, err := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	if err != nil {
		m.Close()
		t.Fatalf("TIOCGPTN: %v", err)
	}
	s, err := os.OpenFile("/dev/pts/"+itoa(n), os.O_RDWR, 0)
	if err != nil {
		m.Close()
		t.Fatalf("open slave: %v", err)
	}
	return m, s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// answerTerminal reads a stream for terminal queries (DA1/CPR/OSC) and writes
// the responses a real terminal would, so bubbletea's startup handshake
// completes instead of blocking until its timeout.
func answerTerminal(r *os.File) {
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, err := r.Read(buf)
		if err != nil {
			return
		}
		pending = append(pending, buf[:n]...)
		var replies []byte
		for {
			idx := bytes.Index(pending, []byte("\x1b[6n")) // cursor position report
			if idx >= 0 {
				replies = append(replies, []byte("\x1b[3;1R")...)
				pending = append(pending[:idx], pending[idx+4:]...)
				continue
			}
			if idx := bytes.Index(pending, []byte("\x1b[c")); idx >= 0 {
				replies = append(replies, []byte("\x1b[?62;22c")...)
				pending = append(pending[:idx], pending[idx+2:]...)
				continue
			}
			if idx := bytes.Index(pending, []byte("\x1b[>c")); idx >= 0 {
				replies = append(replies, []byte("\x1b[>1;10;0c")...)
				pending = append(pending[:idx], pending[idx+3:]...)
				continue
			}
			if idx := bytes.Index(pending, []byte("\x1b]11;?")); idx >= 0 {
				replies = append(replies, []byte("\x1b]11;rgb:1e1e/1e1e/1e1e\x1b\\")...)
				pending = append(pending[:idx], pending[idx+6:]...)
				continue
			}
			if idx := bytes.Index(pending, []byte("\x1b]10;?")); idx >= 0 {
				replies = append(replies, []byte("\x1b]10;rgb:ffff/ffff/ffff\x1b\\")...)
				pending = append(pending[:idx], pending[idx+6:]...)
				continue
			}
			break
		}
		if len(replies) > 0 {
			if _, err := r.Write(replies); err != nil {
				return
			}
		}
	}
}

// TestRunSpinnerExitsWhenTerminalDies guards against the orphaned-process bug:
// if the terminal (stdin) goes away while a spinner is running a long task, the
// spinner must return promptly instead of re-running the task synchronously
// (which would hang forever on a network call and leave the process orphaned).
func TestRunSpinnerExitsWhenTerminalDies(t *testing.T) {
	master, slave := openPty(t)
	defer master.Close()
	defer slave.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldTerm := os.Getenv("TERM")
	// In a real terminal both stdin and stdout are the pty slave; the master is
	// owned by the terminal emulator. mirror that here.
	os.Stdin = slave
	os.Stdout = slave
	os.Setenv("TERM", "xterm-256color")
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Setenv("TERM", oldTerm)
	}()

	go answerTerminal(master)

	// A task that never completes — simulates a hanging network fetch.
	done := make(chan struct{})
	defer close(done)
	task := func() (any, error) {
		<-done // block forever
		return nil, nil
	}

	returned := make(chan struct{})
	go func() {
		runSpinner("test", false, task)
		close(returned)
	}()

	// Let the spinner program start and finish its handshake, then kill the
	// terminal (close the master) so reads on the slave return EIO.
	time.Sleep(500 * time.Millisecond)
	master.Close()

	select {
	case <-returned:
		// runSpinner returned promptly instead of hanging on the task.
	case <-time.After(5 * time.Second):
		t.Fatal("runSpinner did not return after the terminal died — it hung on the task")
	}
}
