//go:build linux

package daemon

import "syscall"

// getDaemonSysProcAttr configures Linux Setsid so the daemon detaches from the controlling terminal.
func getDaemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
