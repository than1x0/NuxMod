package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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

type screenMode int

const (
	modeDashboard screenMode = iota
	modeAddProfile
	modeDeleteProfile
	modeSwitchProfile
	modeLogs
)

// RunDashboard runs the interactive terminal dashboard.
func RunDashboard(profMgr *profile.Manager) {
	scanner := bufio.NewScanner(os.Stdin)
	currentMode := modeDashboard

	// Channel for user inputs from single scanner
	inputChan := make(chan string)
	go func() {
		for scanner.Scan() {
			inputChan <- strings.TrimSpace(scanner.Text())
		}
		close(inputChan)
	}()

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	// Initial render
	renderDashboard(profMgr)

	for {
		select {
		case line, ok := <-inputChan:
			if !ok {
				return
			}
			switch currentMode {
			case modeDashboard:
				cmd := strings.ToLower(line)
				switch cmd {
				case "c":
					handleConnectToggle(profMgr)
					renderDashboard(profMgr)

				case "p":
					currentMode = modeSwitchProfile
					promptSwitchProfile(profMgr)

				case "a":
					currentMode = modeAddProfile
					promptAddProfile()

				case "d":
					currentMode = modeDeleteProfile
					promptDeleteProfile(profMgr)

				case "l":
					currentMode = modeLogs
					showLogsScreen()

				case "r", "":
					renderDashboard(profMgr)

				case "q":
					fmt.Print("\n" + Yellow + "[*] Dashboard closed. VPN is still running in the background!" + Reset + "\n")
					fmt.Print(Cyan + "[*] Run 'sudo vless-linux' at any time to reopen the dashboard." + Reset + "\n\n")
					return

				case "x":
					fmt.Print("\n" + Red + "[*] Disconnecting VPN and stopping background service..." + Reset + "\n")
					_ = daemon.StopBackgroundDaemon()
					fmt.Print(Green + "[+] System routes cleanly restored. Goodbye!" + Reset + "\n\n")
					return

				default:
					fmt.Printf("\033[33mUnknown choice '%s'. Press [c, p, a, d, l, q, x]: \033[0m", line)
				}

			case modeAddProfile:
				if strings.ToLower(line) == "c" || strings.ToLower(line) == "cancel" || line == "" {
					currentMode = modeDashboard
					renderDashboard(profMgr)
					continue
				}
				processAddProfile(profMgr, line, scanner)
				currentMode = modeDashboard
				renderDashboard(profMgr)

			case modeDeleteProfile:
				if strings.ToLower(line) == "c" || strings.ToLower(line) == "cancel" || line == "" {
					currentMode = modeDashboard
					renderDashboard(profMgr)
					continue
				}
				processDeleteProfile(profMgr, line)
				currentMode = modeDashboard
				renderDashboard(profMgr)

			case modeSwitchProfile:
				if strings.ToLower(line) == "c" || strings.ToLower(line) == "cancel" || line == "" {
					currentMode = modeDashboard
					renderDashboard(profMgr)
					continue
				}
				processSwitchProfile(profMgr, line)
				currentMode = modeDashboard
				renderDashboard(profMgr)

			case modeLogs:
				// Any key returns from logs to dashboard
				currentMode = modeDashboard
				renderDashboard(profMgr)
			}

		case <-ticker.C:
			// Live update only while in main dashboard mode
			if currentMode == modeDashboard {
				renderDashboard(profMgr)
			}
		}
	}
}

func renderDashboard(profMgr *profile.Manager) {
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

	// 6. Actions Bar
	fmt.Println(Cyan + Bold + "Actions:" + Reset)
	fmt.Printf("  %s[c]%s Connect/Disconnect  %s[p]%s Switch Profile  %s[a]%s Add Profile  %s[d]%s Delete Profile\n",
		Yellow+Bold, Reset, Yellow+Bold, Reset, Yellow+Bold, Reset, Yellow+Bold, Reset)
	fmt.Printf("  %s[l]%s Full Logs           %s[q]%s Exit (Keep VPN Alive)   %s[x]%s Disconnect & Exit\n",
		Yellow+Bold, Reset, Green+Bold, Reset, Red+Bold, Reset)
	fmt.Print("\nEnter choice: ")
}

func handleConnectToggle(profMgr *profile.Manager) {
	if daemon.IsDaemonRunning() {
		fmt.Println("\n\033[33m[*] Disconnecting VPN...\033[0m")
		_ = daemon.StopBackgroundDaemon()
		time.Sleep(500 * time.Millisecond)
	} else {
		activeProf, err := profMgr.GetActive()
		if err != nil {
			fmt.Println("\n\033[31m[-] No profile configured. Please add one first with [a]!\033[0m")
			time.Sleep(1500 * time.Millisecond)
			return
		}
		fmt.Printf("\n\033[32m[*] Spawning background VPN daemon for [%s]...\033[0m\n", activeProf.Name)
		if err := daemon.StartBackgroundDaemon(); err != nil {
			fmt.Printf("\033[31m[-] Failed to start VPN: %v\033[0m\n", err)
			time.Sleep(2 * time.Second)
			return
		}
		fmt.Println("\033[32m[+] Connected successfully!\033[0m")
		time.Sleep(800 * time.Millisecond)
	}
}

func promptAddProfile() {
	fmt.Print(ClearScreen)
	fmt.Println(Cyan + Bold + "=== Add New VLESS Profile ===" + Reset)
	fmt.Println("Paste your VLESS link below (or type 'c' to cancel):")
	fmt.Print("\nLink: ")
}

func processAddProfile(profMgr *profile.Manager, link string, scanner *bufio.Scanner) {
	link = strings.TrimSpace(link)
	if link == "" || strings.ToLower(link) == "c" || strings.ToLower(link) == "cancel" {
		return
	}

	fmt.Print("Profile Name (optional, press Enter for auto): ")
	var name string
	if scanner.Scan() {
		name = strings.TrimSpace(scanner.Text())
	}

	prof, err := profMgr.Add(link, name)
	if err != nil {
		fmt.Printf("\n\033[31m[-] Failed to add profile: %v\033[0m\n", err)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Printf("\n\033[32m[+] Profile [%s] added successfully!\033[0m\n", prof.Name)
	time.Sleep(1 * time.Second)
}

func promptDeleteProfile(profMgr *profile.Manager) {
	fmt.Print(ClearScreen)
	fmt.Println(Cyan + Bold + "=== Delete Profile ===" + Reset)
	profiles := profMgr.List()
	for i, p := range profiles {
		fmt.Printf("  %d. %s (%s:%d)\n", i+1, p.Name, p.Server, p.Port)
	}
	fmt.Print("\nEnter profile number to delete (or 'c' to cancel): ")
}

func processDeleteProfile(profMgr *profile.Manager, input string) {
	input = strings.TrimSpace(input)
	if input == "" || strings.ToLower(input) == "c" || strings.ToLower(input) == "cancel" {
		return
	}

	target := input
	profiles := profMgr.List()
	if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(profiles) {
		target = profiles[num-1].ID
	}

	if err := profMgr.Delete(target); err != nil {
		fmt.Printf("\n\033[31m[-] Delete error: %v\033[0m\n", err)
		time.Sleep(2 * time.Second)
	} else {
		fmt.Println("\n\033[32m[+] Profile deleted.\033[0m")
		time.Sleep(1 * time.Second)
	}
}

func promptSwitchProfile(profMgr *profile.Manager) {
	fmt.Print(ClearScreen)
	fmt.Println(Cyan + Bold + "=== Switch Active Profile ===" + Reset)
	profiles := profMgr.List()
	for i, p := range profiles {
		tag := ""
		if p.Active {
			tag = Green + " [ACTIVE]" + Reset
		}
		fmt.Printf("  %d. %s (%s:%d)%s\n", i+1, p.Name, p.Server, p.Port, tag)
	}
	fmt.Print("\nEnter profile number to activate (or 'c' to cancel): ")
}

func processSwitchProfile(profMgr *profile.Manager, input string) {
	input = strings.TrimSpace(input)
	if input == "" || strings.ToLower(input) == "c" || strings.ToLower(input) == "cancel" {
		return
	}

	target := input
	profiles := profMgr.List()
	if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(profiles) {
		target = profiles[num-1].ID
	}

	selected, err := profMgr.SetActive(target)
	if err != nil {
		fmt.Printf("\n\033[31m[-] Error: %v\033[0m\n", err)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Printf("\n\033[32m[+] Active profile set to [%s]!\033[0m\n", selected.Name)

	// If daemon is running, reconnect it with the new profile
	if daemon.IsDaemonRunning() {
		fmt.Println("\033[33m[*] Reconnecting daemon with new profile...\033[0m")
		_ = daemon.StopBackgroundDaemon()
		time.Sleep(500 * time.Millisecond)
		_ = daemon.StartBackgroundDaemon()
	}
	time.Sleep(1 * time.Second)
}

func showLogsScreen() {
	resp, err := daemon.SendIPCCommand(daemon.IPCRequest{Command: "LOGS"}, 2*time.Second)
	fmt.Print(ClearScreen)
	fmt.Println(Cyan + Bold + "=== Full Event Logs (Last 50 Entries) ===" + Reset)
	if err != nil || resp == nil || len(resp.Logs) == 0 {
		fmt.Println("\n\033[33m[No logs available or VPN not running]\033[0m")
	} else {
		for _, l := range resp.Logs {
			fmt.Println(l)
		}
	}
	fmt.Println("\n\033[36mPress Enter to return to Dashboard...\033[0m")
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
