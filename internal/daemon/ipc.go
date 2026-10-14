package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	DefaultSocketPath = "/var/run/nuxmod.sock"
	LegacySocketPath  = "/var/run/vless-linux.sock"
	FallbackSocketDir = "/tmp"
	DefaultPIDPath    = "/var/run/nuxmod.pid"
)

func GetSocketPath() string {
	if _, err := os.Stat(DefaultSocketPath); err == nil {
		return DefaultSocketPath
	}
	if _, err := os.Stat(LegacySocketPath); err == nil {
		return LegacySocketPath
	}
	if os.Geteuid() == 0 {
		return DefaultSocketPath
	}
	return filepath.Join(FallbackSocketDir, fmt.Sprintf("nuxmod-%d.sock", os.Getuid()))
}

func GetPIDPath() string {
	if os.Geteuid() == 0 {
		return DefaultPIDPath
	}
	return filepath.Join(FallbackSocketDir, fmt.Sprintf("nuxmod-%d.pid", os.Getuid()))
}

// IPCRequest represents a request sent from CLI/TUI to the Daemon.
type IPCRequest struct {
	Command string `json:"command"` // "STATUS", "DISCONNECT", "SWITCH", "LOGS", "PING"
	Payload string `json:"payload,omitempty"`
}

// IPCResponse represents the Daemon's reply to the CLI/TUI.
type IPCResponse struct {
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
	Status  *DaemonStatus `json:"status,omitempty"`
	Logs    []string      `json:"logs,omitempty"`
}

// DaemonStatus represents real-time operational status.
type DaemonStatus struct {
	Running       bool      `json:"running"`
	ProfileName   string    `json:"profile_name"`
	Server        string    `json:"server"`
	Port          int       `json:"port"`
	SNI           string    `json:"sni"`
	ConnectedAt   time.Time `json:"connected_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	RxSpeed       float64   `json:"rx_speed"` // bytes/sec
	TxSpeed       float64   `json:"tx_speed"` // bytes/sec
	TotalRx       uint64    `json:"total_rx"` // bytes
	TotalTx       uint64    `json:"total_tx"` // bytes
	LatencyMs     int64     `json:"latency_ms"`
	RecentLogs    []string  `json:"recent_logs"`
}

// SendIPCCommand connects to the running daemon and issues a command.
func SendIPCCommand(req IPCRequest, timeout time.Duration) (*IPCResponse, error) {
	sockPath := GetSocketPath()
	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("daemon is not running or socket unreachable: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send IPC request: %w", err)
	}

	var resp IPCResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read IPC response: %w", err)
	}

	return &resp, nil
}

// IsDaemonRunning checks if the daemon socket is alive and responding.
func IsDaemonRunning() bool {
	resp, err := SendIPCCommand(IPCRequest{Command: "PING"}, 500*time.Millisecond)
	return err == nil && resp != nil && resp.Success
}

// StartBackgroundDaemon spawns the daemon in a detached background process.
func StartBackgroundDaemon() error {
	if IsDaemonRunning() {
		return nil
	}
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "/usr/local/bin/nuxmod"
	}

	logPath := "/var/log/nuxmod.log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFile, _ = os.OpenFile("/tmp/nuxmod.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}

	cmd := exec.Command(selfPath, "__daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // Detach from terminal
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn daemon process: %w", err)
	}

	// Wait up to 5 seconds for socket to respond
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if IsDaemonRunning() {
			return nil
		}
	}
	return fmt.Errorf("daemon process started but IPC socket is not ready yet (check /var/log/nuxmod.log)")
}

// StopBackgroundDaemon disconnects the running daemon cleanly.
func StopBackgroundDaemon() error {
	if !IsDaemonRunning() {
		return nil
	}
	_, err := SendIPCCommand(IPCRequest{Command: "DISCONNECT"}, 3*time.Second)
	if err != nil {
		_ = exec.Command("killall", "-9", "sing-box", "nuxmod", "vless-linux").Run()
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}
