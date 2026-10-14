//go:build !linux

package daemon

import "syscall"

// getDaemonSysProcAttr returns default process attributes on non-Linux platforms for compatibility.
func getDaemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
