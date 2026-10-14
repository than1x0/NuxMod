//go:build linux

package tui

import (
	"syscall"
	"unsafe"
)

// getTermSizeIoctl performs Linux TIOCGWINSZ ioctl syscall to query terminal window dimensions.
func getTermSizeIoctl(fd uintptr, ws *winsize) bool {
	ret, _, _ := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(0x5413), uintptr(unsafe.Pointer(ws)))
	return int(ret) == 0 && ws.Col > 0 && ws.Row > 0
}
