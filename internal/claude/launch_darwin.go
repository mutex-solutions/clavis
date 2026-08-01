//go:build darwin

package claude

import "syscall"

func setPlatformProcAttr() *syscall.SysProcAttr {
	// Pdeathsig not available on Darwin.
	return &syscall.SysProcAttr{}
}
