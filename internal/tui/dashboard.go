package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/daemon"
	"github.com/Sathira-Laksahan/vless-linux/internal/profile"
)

// ANSI Color Codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	ClearScreen = "\033[H\033[2J"
)

// RunDashboard starts the live terminal dashboard loop.
func RunDashboard(profMgr *profile.Manager) {
	// Check if terminal input is available
	inputChan := make(chan string)
	go readKeyInputs(inputChan)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		renderView(profMgr)

		select {
		case key := <-inputChan:
			switch strings.ToLower(key) {
			case "q":
				fmt.Print("\n\033[33m[*] Dashboard closed. VPN connection is still running in the background!\033[0m\n")
				fmt.Print("\033[36m[*] Run 'sudo vless-linux' at any time to reopen the dashboard.\033[0m\n\n")
				return

			case "x":
				fmt.Print("\n\033[31m[*] Disconnecting and stopping VPN...\033[0m\n")
				_, _ = daemon.SendIPCCommand(daemon.IPCRequest{Command: "DISCONNECT"}, 2*time.Second)
				fmt.Print("\033[32m[+] Network routes restored cleanly. Goodbye!\033[0m\n\n")
				return

			case "c":
				// Toggle connection
				handleConnectToggle(profMgr)

			case "a":
				handleAddProfile(profMgr)

			case "d":
				handleDeleteProfile(profMgr)

			case "p":
				handleSwitchProfile(profMgr)

			case "l":
				showFullLogs()
			}

		case <-ticker.C:
			// Next tick renders dashboard
		}
	}
}

func renderView(profMgr *profile.Manager) {
	fmt.Print(ClearScreen)

	// Fetch daemon status over IPC
	resp, _ := daemon.SendIPCCommand(daemon.IPCRequest{Command: "STATUS"}, 500*time.Millisecond)
	var status *daemon.DaemonStatus
	if resp != nil && resp.Success {
		status = resp.Status
	}

	activeProf, _ := profMgr.GetActive()

	// 1. Header Box
	fmt.Println(Cyan + Bold + "╔══════════════════════════════════════════════════════════════════════════════╗" + Reset)
	fmt.Println(Cyan + Bold + "║                     VLESS LINUX BYPASS - TUN DASHBOARD                       ║" + Reset)
	fmt.Println(Cyan + Bold + "╚══════════════════════════════════════════════════════════════════════════════╝" + Reset)

	// 2. Status Box
	fmt.Println(White + Bold + "┌─ Connection Status ──────────────────────────────────────────────────────────┐" + Reset)
	if status != nil && status.Running {
		uptimeStr := formatDuration(time.Duration(status.UptimeSeconds) * time.Second)
		latStr := fmt.Sprintf("%d ms", status.LatencyMs)
		if status.LatencyMs <= 0 {
			latStr = "testing..."
		}

		fmt.Printf("│  Status:  %s%s● CONNECTED%s       Active Node: %s%s%s\n", Green, Bold, Reset, Yellow, status.ProfileName, Reset)
		fmt.Printf("│  Server:  %s:%d  (SNI: %s%s%s)\n", status.Server, status.Port, Cyan, status.SNI, Reset)
		fmt.Printf("│  Latency: %s%s%s           Uptime: %s      TUN Device: %stun0%s\n", Green, latStr, Reset, uptimeStr, Cyan, Reset)
	} else {
		nodeName := "None"
		if activeProf != nil {
			nodeName = activeProf.Name
		}
		fmt.Printf("│  Status:  %s%s○ DISCONNECTED%s    Selected Node: %s%s%s\n", Red, Bold, Reset, Yellow, nodeName, Reset)
		fmt.Printf("│  Server:  %s\n", func() string {
			if activeProf != nil {
				return fmt.Sprintf("%s:%d", activeProf.Server, activeProf.Port)
			}
			return "No server configured"
		}())
		fmt.Println("│  Press [c] to connect the VPN now.")
	}
	fmt.Println(White + Bold + "└──────────────────────────────────────────────────────────────────────────────┘" + Reset)

	// 3. Speedometer & Bandwidth Box
	fmt.Println(White + Bold + "┌─ Real-Time Bandwidth Monitor ────────────────────────────────────────────────┐" + Reset)
	if status != nil && status.Running {
		rxBar := renderBar(status.RxSpeed, 5*1024*1024)
		txBar := renderBar(status.TxSpeed, 2*1024*1024)

		fmt.Printf("│  DOWNLOAD: %s%10s%s  %s  Total: %s\n", Green+Bold, formatBytes(status.RxSpeed)+"/s", Reset, rxBar, formatBytes(float64(status.TotalRx)))
		fmt.Printf("│  UPLOAD:   %s%10s%s  %s  Total: %s\n", Blue+Bold, formatBytes(status.TxSpeed)+"/s", Reset, txBar, formatBytes(float64(status.TotalTx)))
	} else {
		fmt.Println("│  [Interface tun0 inactive. Connect VPN to start monitoring data transfer.]  ")
	}
	fmt.Println(White + Bold + "└──────────────────────────────────────────────────────────────────────────────┘" + Reset)

	// 4. Saved Profiles Box
	fmt.Println(White + Bold + "┌─ Saved Profiles ─────────────────────────────────────────────────────────────┐" + Reset)
	profiles := profMgr.List()
	if len(profiles) == 0 {
		fmt.Println("│  No profiles saved yet. Press [a] to add a new VLESS link!                   │")
	} else {
		for i, p := range profiles {
			marker := " "
			tag := ""
			if (status != nil && status.ProfileName == p.Name) || (status == nil && p.Active) {
				marker = "*"
				tag = Green + "[ACTIVE]" + Reset
			}
			fmt.Printf("│  [%s] %d. %-22s %-32s %s\n", marker, i+1, truncate(p.Name, 22), truncate(fmt.Sprintf("%s:%d", p.Server, p.Port), 32), tag)
		}
	}
	fmt.Println(White + Bold + "└──────────────────────────────────────────────────────────────────────────────┘" + Reset)

	// 5. Mini Event Logs Box
	fmt.Println(White + Bold + "┌─ Recent System Logs ─────────────────────────────────────────────────────────┐" + Reset)
	if status != nil && len(status.RecentLogs) > 0 {
		for _, logLine := range status.RecentLogs {
			fmt.Printf("│  %s\n", truncate(logLine, 74))
		}
	} else {
		fmt.Println("│  [No active logs available. Connection idle.]                                │")
	}
	fmt.Println(White + Bold + "└──────────────────────────────────────────────────────────────────────────────┘" + Reset)

	// 6. Interactive Actions Bar
	fmt.Println(Cyan + Bold + "Actions:" + Reset)
	fmt.Printf("  %s[c]%s Connect/Disconnect  %s[p]%s Switch Profile  %s[a]%s Add Profile  %s[d]%s Delete Profile\n",
		Yellow+Bold, Reset, Yellow+Bold, Reset, Yellow+Bold, Reset, Yellow+Bold, Reset)
	fmt.Printf("  %s[q]%s Exit to Terminal (Keep VPN Running)     %s[x]%s Disconnect & Exit\n",
		Green+Bold, Reset, Red+Bold, Reset)
	fmt.Print("\nEnter choice: ")
}

func handleConnectToggle(profMgr *profile.Manager) {
	if daemon.IsDaemonRunning() {
		fmt.Print("\n\033[33m[*] Disconnecting...\033[0m")
		_, _ = daemon.SendIPCCommand(daemon.IPCRequest{Command: "DISCONNECT"}, 2*time.Second)
		time.Sleep(500 * time.Millisecond)
	} else {
		prof, err := profMgr.GetActive()
		if err != nil {
			fmt.Println("\n\033[31m[-] No active profile to connect. Please add one first with [a]!\033[0m")
			time.Sleep(1 * time.Second)
			return
		}
		fmt.Printf("\n\033[32m[*] Starting background connection to [%s]...\033[0m\n", prof.Name)
		go func() {
			d := daemon.NewDaemon(profMgr)
			_ = d.Run(nil, prof)
		}()
		time.Sleep(1 * time.Second)
	}
}

func handleAddProfile(profMgr *profile.Manager) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n\033[36mEnter VLESS Link (vless://...):\033[0m ")
	link, _ := reader.ReadString('\n')
	link = strings.TrimSpace(link)
	if link == "" {
		return
	}

	fmt.Print("\033[36mEnter Profile Name (optional, press Enter for auto):\033[0m ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	prof, err := profMgr.Add(link, name)
	if err != nil {
		fmt.Printf("\n\033[31m[-] Failed to add profile: %v\033[0m\n", err)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Printf("\n\033[32m[+] Profile [%s] added successfully!\033[0m\n", prof.Name)
	time.Sleep(1 * time.Second)
}

func handleDeleteProfile(profMgr *profile.Manager) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n\033[33mEnter Profile Name or Number to delete:\033[0m ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Check if user entered a number
	profiles := profMgr.List()
	for i, p := range profiles {
		if fmt.Sprintf("%d", i+1) == input {
			input = p.ID
			break
		}
	}

	if err := profMgr.Delete(input); err != nil {
		fmt.Printf("\n\033[31m[-] Error deleting profile: %v\033[0m\n", err)
	} else {
		fmt.Println("\n\033[32m[+] Profile deleted successfully.\033[0m")
	}
	time.Sleep(1 * time.Second)
}

func handleSwitchProfile(profMgr *profile.Manager) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n\033[36mEnter Profile Number to activate:\033[0m ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	profiles := profMgr.List()
	var targetID string
	for i, p := range profiles {
		if fmt.Sprintf("%d", i+1) == input || p.Name == input {
			targetID = p.ID
			break
		}
	}

	if targetID == "" {
		fmt.Println("\n\033[31m[-] Invalid profile number.\033[0m")
		time.Sleep(1 * time.Second)
		return
	}

	selected, err := profMgr.SetActive(targetID)
	if err != nil {
		fmt.Printf("\n\033[31m[-] Error: %v\033[0m\n", err)
		time.Sleep(1 * time.Second)
		return
	}

	fmt.Printf("\n\033[32m[+] Active profile switched to [%s]!\033[0m\n", selected.Name)

	// If daemon is running, reconnect with new profile
	if daemon.IsDaemonRunning() {
		fmt.Println("\033[33m[*] Reconnecting with new profile...\033[0m")
		_, _ = daemon.SendIPCCommand(daemon.IPCRequest{Command: "DISCONNECT"}, 2*time.Second)
		time.Sleep(1 * time.Second)
		go func() {
			d := daemon.NewDaemon(profMgr)
			_ = d.Run(nil, selected)
		}()
	}
	time.Sleep(1 * time.Second)
}

func showFullLogs() {
	resp, err := daemon.SendIPCCommand(daemon.IPCRequest{Command: "LOGS"}, 2*time.Second)
	if err != nil || resp == nil || len(resp.Logs) == 0 {
		fmt.Println("\n\033[33mNo logs available.\033[0m")
		time.Sleep(1 * time.Second)
		return
	}

	fmt.Print(ClearScreen)
	fmt.Println(Cyan + Bold + "=== Full Event Logs (Last 50 Entries) ===" + Reset)
	for _, l := range resp.Logs {
		fmt.Println(l)
	}
	fmt.Println("\nPress Enter to return to Dashboard...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func readKeyInputs(out chan string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			out <- text
		}
	}
}

func renderBar(currentBytes float64, maxBytes float64) string {
	const barWidth = 16
	if maxBytes <= 0 {
		maxBytes = 1
	}
	ratio := currentBytes / maxBytes
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

func formatBytes(bytes float64) string {
	const unit = 1024.0
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", bytes/div, "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
