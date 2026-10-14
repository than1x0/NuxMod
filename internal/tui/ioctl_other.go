//go:build !linux

package tui

// getTermSizeIoctl is a fallback stub for non-Linux operating systems.
func getTermSizeIoctl(fd uintptr, ws *winsize) bool {
	return false
}
