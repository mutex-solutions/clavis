//go:build windows

package claude

import (
	"os"
	"syscall"
)

// setPlatformProcAttr returns default attrs. Windows has no Pdeathsig/Setpgid
// equivalent here; the child shares the console for interactive use.
func setPlatformProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// forwardSignal relays a signal to the claude process. On Windows only os.Kill
// is meaningful for a child process; os.Interrupt cannot be sent to another
// process, so we best-effort forward and ignore unsupported signals.
func forwardSignal(p *os.Process, sig os.Signal) {
	if sig == os.Kill {
		_ = p.Kill()
	}
}

// exitCode extracts the exit code from a wait error's platform status. Windows
// has no signal-death concept, so we return the raw exit status.
func exitCode(sys any) (int, bool) {
	status, ok := sys.(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	return status.ExitStatus(), true
}
