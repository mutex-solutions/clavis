//go:build darwin

package claude

import (
	"os"
	"syscall"
)

// setPlatformProcAttr returns default attrs. Pdeathsig is not available on
// Darwin, and Setpgid would break the interactive TUI (SIGTTIN).
func setPlatformProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// forwardSignal relays a signal to the claude process.
func forwardSignal(p *os.Process, sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok {
		_ = p.Signal(s)
	}
}

// exitCode extracts the shell-conventional exit code from a wait error's
// platform status: 128+signal for signal death, otherwise the raw code.
func exitCode(sys any) (int, bool) {
	status, ok := sys.(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	if status.Signaled() {
		return 128 + int(status.Signal()), true
	}
	return status.ExitStatus(), true
}
