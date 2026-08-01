//go:build linux

package claude

import (
	"os"
	"syscall"
)

// setPlatformProcAttr asks the kernel to SIGTERM claude if clavis dies
// (Linux-only Pdeathsig). We deliberately do NOT set Setpgid: claude is an
// interactive TUI that needs to own the controlling terminal, and putting it in
// a new process group without terminal ownership makes it stop on SIGTTIN.
func setPlatformProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
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
