package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/config"
	"github.com/Sathira-Laksahan/vless-linux/internal/engine"
	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
	"github.com/Sathira-Laksahan/vless-linux/internal/profile"
	"github.com/Sathira-Laksahan/vless-linux/internal/routing"
)

const (
	MaxLogLines = 100
)

// Daemon handles the background lifecycle, TUN routing, metrics, and IPC socket.
type Daemon struct {
	mu            sync.RWMutex
	profileMgr    *profile.Manager
	activeProfile *profile.Profile
	engineMgr     *engine.EngineManager

	listener net.Listener
	stopChan chan struct{}

	connectedAt time.Time
	rxSpeed     float64 // bytes/sec
	txSpeed     float64 // bytes/sec
	totalRx     uint64  // total bytes
	totalTx     uint64  // total bytes
	prevRx      uint64
	prevTx      uint64
	prevSample  time.Time
	latencyMs   int64

	logBuffer []string
	logMu     sync.RWMutex
}

func NewDaemon(profMgr *profile.Manager) *Daemon {
	return &Daemon{
		profileMgr: profMgr,
		stopChan:   make(chan struct{}),
		logBuffer:  make([]string, 0, MaxLogLines),
	}
}

// Write implements io.Writer to intercept sing-box logs into memory ring buffer.
func (d *Daemon) Write(p []byte) (n int, err error) {
	text := strings.TrimSpace(string(p))
	if text == "" {
		return len(p), nil
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		d.addLog(line)
	}
	return len(p), nil
}

func (d *Daemon) addLog(line string) {
	d.logMu.Lock()
	defer d.logMu.Unlock()

	// Strip raw ANSI escape sequences from sing-box
	clean := stripAnsi(line)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return
	}

	// Filter and format message
	if idx := strings.Index(clean, "INFO "); idx != -1 {
		clean = strings.TrimSpace(clean[idx+5:])
	} else if idx := strings.Index(clean, "WARN "); idx != -1 {
		clean = strings.TrimSpace(clean[idx+5:])
	} else if idx := strings.Index(clean, "ERROR "); idx != -1 {
		clean = strings.TrimSpace(clean[idx+6:])
	}

	cleanLine := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), clean)

	if len(d.logBuffer) >= MaxLogLines {
		d.logBuffer = d.logBuffer[1:]
	}
	d.logBuffer = append(d.logBuffer, cleanLine)
}

func stripAnsi(str string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range str {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func (d *Daemon) getRecentLogs(count int) []string {
	d.logMu.RLock()
	defer d.logMu.RUnlock()

	total := len(d.logBuffer)
	if total == 0 {
		return []string{"[Engine initialized, waiting for traffic...]"}
	}
	if count > total {
		count = total
	}
	result := make([]string, count)
	copy(result, d.logBuffer[total-count:])
	return result
}

// Run starts the daemon with the specified profile.
func (d *Daemon) Run(ctx context.Context, prof *profile.Profile) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if prof == nil {
		var err error
		prof, err = d.profileMgr.GetActive()
		if err != nil {
			return fmt.Errorf("no profile available to run daemon: %w", err)
		}
	}
	d.activeProfile = prof

	sockPath := GetSocketPath()
	pidPath := GetPIDPath()

	// Clean up old socket if dead
	_ = os.Remove(sockPath)
	_ = os.Remove(pidPath)

	// Write PID file
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidPath)

	// Start Unix socket listener
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("failed to bind IPC socket %s: %w", sockPath, err)
	}
	_ = os.Chmod(sockPath, 0666) // Allow user access
	d.listener = listener
	defer listener.Close()
	defer os.Remove(sockPath)

	// Clean up any stale sing-box and disable IPv6
	if runtime.GOOS == "linux" {
		_ = exec.Command("killall", "-9", "sing-box").Run()
		setIPv6Disabled(true)
	}
	defer setIPv6Disabled(false)

	// Parse VLESS link
	vlessCfg, err := parser.ParseVlessLink(prof.URL)
	if err != nil {
		return fmt.Errorf("invalid VLESS profile link: %w", err)
	}

	// Generate Sing-box config
	configDir := "/etc/vless-linux"
	if os.Geteuid() != 0 {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".vless-linux")
	}
	_ = os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")

	singboxCfg, err := config.GenerateSingBoxConfig(vlessCfg)
	if err != nil {
		return fmt.Errorf("failed to generate sing-box config: %w", err)
	}
	if err := config.SaveConfigToFile(singboxCfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Ensure core binary
	d.engineMgr = engine.NewEngineManager(configPath)
	binDir := filepath.Join(configDir, "bin")
	if _, err := d.engineMgr.EnsureBinary(binDir); err != nil {
		return fmt.Errorf("sing-box core error: %w", err)
	}

	d.connectedAt = time.Now()
	d.addLog(fmt.Sprintf("System-Wide TUN Bypass started for [%s]", prof.Name))

	// Start Sing-box engine in background, piping output to memory log buffer
	engineCtx, engineCancel := context.WithCancel(ctx)
	defer engineCancel()

	if err := d.engineMgr.Start(engineCtx, d, d); err != nil {
		return fmt.Errorf("failed to start sing-box engine: %w", err)
	}

	// Start background metric and latency monitors
	go d.trafficMonitorLoop(engineCtx)
	go d.latencyMonitorLoop(engineCtx, vlessCfg.Server, vlessCfg.Port)

	// Start IPC server loop
	go d.ipcServerLoop()

	// Wait for stop signal or context cancellation
	select {
	case <-ctx.Done():
	case <-d.stopChan:
	}

	d.addLog("Stopping engine and restoring network routes...")
	_ = d.engineMgr.Stop()
	return nil
}

func (d *Daemon) Stop() {
	select {
	case <-d.stopChan:
	default:
		close(d.stopChan)
	}
}

func (d *Daemon) ipcServerLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			return
		}
		go d.handleIPCConnection(conn)
	}
}

func (d *Daemon) handleIPCConnection(conn net.Conn) {
	defer conn.Close()

	var req IPCRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	resp := IPCResponse{Success: true}

	switch req.Command {
	case "PING":
		resp.Success = true

	case "STATUS":
		d.mu.RLock()
		uptime := int64(time.Since(d.connectedAt).Seconds())
		status := &DaemonStatus{
			Running:       true,
			ProfileName:   d.activeProfile.Name,
			Server:        d.activeProfile.Server,
			Port:          d.activeProfile.Port,
			SNI:           d.activeProfile.SNI,
			ConnectedAt:   d.connectedAt,
			UptimeSeconds: uptime,
			RxSpeed:       d.rxSpeed,
			TxSpeed:       d.txSpeed,
			TotalRx:       d.totalRx,
			TotalTx:       d.totalTx,
			LatencyMs:     d.latencyMs,
			RecentLogs:    d.getRecentLogs(6),
		}
		d.mu.RUnlock()
		resp.Status = status

	case "LOGS":
		resp.Logs = d.getRecentLogs(50)

	case "DISCONNECT":
		resp.Success = true
		_ = json.NewEncoder(conn).Encode(resp)
		d.Stop()
		return

	default:
		resp.Success = false
		resp.Error = "Unknown command"
	}

	_ = json.NewEncoder(conn).Encode(resp)
}

func (d *Daemon) trafficMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	d.prevSample = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rx, tx := readTunStatistics()
			now := time.Now()
			duration := now.Sub(d.prevSample).Seconds()
			if duration <= 0 {
				duration = 1
			}

			d.mu.Lock()
			if d.prevRx > 0 && rx >= d.prevRx {
				d.rxSpeed = float64(rx-d.prevRx) / duration
			} else {
				d.rxSpeed = 0
			}
			if d.prevTx > 0 && tx >= d.prevTx {
				d.txSpeed = float64(tx-d.prevTx) / duration
			} else {
				d.txSpeed = 0
			}
			d.totalRx = rx
			d.totalTx = tx
			d.prevRx = rx
			d.prevTx = tx
			d.prevSample = now
			d.mu.Unlock()
		}
	}
}

func (d *Daemon) latencyMonitorLoop(ctx context.Context, server string, port int) {
	// Initial test
	if lat, err := routing.TestLatency(server, port, 3*time.Second); err == nil {
		d.mu.Lock()
		d.latencyMs = lat.Milliseconds()
		d.mu.Unlock()
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if lat, err := routing.TestLatency(server, port, 3*time.Second); err == nil {
				d.mu.Lock()
				d.latencyMs = lat.Milliseconds()
				d.mu.Unlock()
			}
		}
	}
}

func readTunStatistics() (uint64, uint64) {
	rx := readUintFromFile("/sys/class/net/tun0/statistics/rx_bytes")
	tx := readUintFromFile("/sys/class/net/tun0/statistics/tx_bytes")
	return rx, tx
}

func readUintFromFile(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func setIPv6Disabled(disabled bool) {
	if runtime.GOOS != "linux" {
		return
	}
	val := "0"
	if disabled {
		val = "1"
	}
	_ = os.WriteFile("/proc/sys/net/ipv6/conf/all/disable_ipv6", []byte(val), 0644)
	_ = os.WriteFile("/proc/sys/net/ipv6/conf/default/disable_ipv6", []byte(val), 0644)
	_ = exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv6.conf.all.disable_ipv6=%s", val)).Run()
}
