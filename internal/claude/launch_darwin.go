//go:build darwin

package claude

import "syscall"

func setPlatformProcAttr() *syscall.SysProcAttr {
	// Pdeathsig not available on Darwin.
	// Setpgid isolates claude in its own process group.
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
