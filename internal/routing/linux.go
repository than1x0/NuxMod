package routing

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// CheckLinuxEnvironment verifies whether the Linux system is prepared for TUN mode.
func CheckLinuxEnvironment() error {
	if runtime.GOOS != "linux" {
		return nil // Bypass if running on non-linux during dev
	}

	// 1. Check Root / Sudo privileges
	if os.Geteuid() != 0 {
		return errors.New("root privileges (sudo) required to configure Linux TUN network interface")
	}

	// 2. Check /dev/net/tun
	if _, err := os.Stat("/dev/net/tun"); os.IsNotExist(err) {
		// Attempt to create /dev/net/tun if missing
		_ = os.MkdirAll("/dev/net", 0755)
		cmd := exec.Command("mknod", "/dev/net/tun", "c", "10", "200")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("/dev/net/tun does not exist and failed to create: %w", err)
		}
		_ = exec.Command("chmod", "666", "/dev/net/tun").Run()
	}

	return nil
}

// TestLatency performs a fast TCP dial to test response time (ping) to a host.
func TestLatency(host string, port int, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	elapsed := time.Since(start)
	return elapsed, nil
}
