package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/daemon"
	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
	"github.com/Sathira-Laksahan/vless-linux/internal/profile"
	"github.com/Sathira-Laksahan/vless-linux/internal/routing"
	"github.com/Sathira-Laksahan/vless-linux/internal/tui"
	"github.com/Sathira-Laksahan/vless-linux/internal/ui"
	"github.com/Sathira-Laksahan/vless-linux/internal/updater"
)

const (
	AppName    = "NuxMod"
	AppVersion = "2.0.0"
	Author     = "Sathira Laksahan"
)

func main() {
	profMgr, err := profile.NewManager()
	if err != nil {
		fmt.Printf("Warning: profile manager initialization: %v\n", err)
	}

	// Default behavior with no arguments: Launch Interactive TUI Dashboard directly!
	if len(os.Args) < 2 {
		checkRootPrivilege()
		tui.RunDashboard(profMgr)
		return
	}

	command := os.Args[1]

	switch command {
	case "dashboard", "dash", "ui":
		checkRootPrivilege()
		tui.RunDashboard(profMgr)

	case "connect", "start":
		checkRootPrivilege()
		handleConnect(profMgr, os.Args[2:])

	case "disconnect", "stop":
		checkRootPrivilege()
		handleDisconnect()

	case "status":
		handleStatus()

	case "profile", "profiles":
		handleProfileCmd(profMgr, os.Args[2:])

	case "logs":
		handleLogs()

	case "update", "upgrade":
		checkRootPrivilege()
		handleUpdate()

	case "__daemon":
		// Internal daemon entrypoint (runs in background detached)
		handleDaemon(profMgr)

	case "parse":
		handleParse(os.Args[2:])

	case "version", "-v", "--version":
		fmt.Printf("%s v%s by %s\n", AppName, AppVersion, Author)

	case "help", "-h", "--help":
		printUsage()

	default:
		// If user passed a vless:// URL directly
		if strings.HasPrefix(command, "vless://") {
			checkRootPrivilege()
			handleConnect(profMgr, []string{command})
		} else {
			ui.Error("Unknown command: %s", command)
			printUsage()
		}
	}
}

func checkRootPrivilege() {
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		fmt.Println("\033[33m[!] Root privileges required for TUN routing. Please run with 'sudo':\033[0m")
		fmt.Printf("    \033[36msudo %s %s\033[0m\n\n", os.Args[0], strings.Join(os.Args[1:], " "))
		os.Exit(1)
	}
}

func printUsage() {
	ui.PrintBanner()
	ui.Header("Usage & Commands")
	fmt.Println("  sudo nuxmod                       Open the interactive live TUI Dashboard (Default)")
	fmt.Println("  sudo nuxmod connect [url|name]    Connect to VPN in background (terminal close won't drop)")
	fmt.Println("  sudo nuxmod disconnect            Disconnect VPN and restore network routes cleanly")
	fmt.Println("  nuxmod status                     Show current connection status and real-time bandwidth")
	fmt.Println("  nuxmod logs                       View recent connection and DNS query logs")
	fmt.Println("  nuxmod profile list               List all saved VLESS node profiles")
	fmt.Println("  nuxmod profile add <url> [name]   Save a new VLESS link as a profile")
	fmt.Println("  nuxmod profile use <name|num>     Switch active profile")
	fmt.Println("  nuxmod profile delete <name|num>  Delete a profile")
	fmt.Println("  sudo nuxmod update                Check and update NuxMod to latest version")
	fmt.Println("  nuxmod version                    Display tool version and author")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sudo nuxmod")
	fmt.Println("  sudo nuxmod connect \"vless://...\"")
	fmt.Println("  sudo nuxmod connect")
	fmt.Println("  sudo nuxmod disconnect")
	fmt.Println()
	fmt.Printf("NuxMod v%s — Built with ❤️ by %s\n\n", AppVersion, Author)
}

func handleConnect(profMgr *profile.Manager, args []string) {
	// If a link was provided as argument, save/activate it
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		arg := strings.TrimSpace(args[0])
		if strings.HasPrefix(arg, "vless://") {
			name := ""
			if len(args) > 1 {
				name = strings.TrimSpace(args[1])
			}
			prof, err := profMgr.Add(arg, name)
			if err != nil {
				ui.Error("Failed to add profile: %v", err)
				os.Exit(1)
			}
			_, _ = profMgr.SetActive(prof.ID)
		} else {
			// Argument is a profile name or number
			profiles := profMgr.List()
			matched := false
			for i, p := range profiles {
				if fmt.Sprintf("%d", i+1) == arg || p.Name == arg || p.ID == arg {
					_, _ = profMgr.SetActive(p.ID)
					matched = true
					break
				}
			}
			if !matched {
				ui.Error("Profile '%s' not found.", arg)
				os.Exit(1)
			}
		}
	}

	activeProf, err := profMgr.GetActive()
	if err != nil {
		ui.Error("No active profile configured. Add one with: sudo nuxmod profile add \"vless://...\"")
		os.Exit(1)
	}

	// Check if daemon is already running
	if daemon.IsDaemonRunning() {
		ui.Success("VPN is already running in background! Connected node: [%s]", activeProf.Name)
		ui.Info("Run 'sudo nuxmod' to view the live dashboard.")
		return
	}

	ui.Info("Connecting to [%s] (%s:%d)...", activeProf.Name, activeProf.Server, activeProf.Port)

	// Environment check
	_ = routing.CheckLinuxEnvironment()

	// Spawn daemon as a detached background process
	if err := daemon.StartBackgroundDaemon(); err != nil {
		ui.Error("Failed to start VPN: %v", err)
		os.Exit(1)
	}

	fmt.Println()
	ui.Success("System-Wide TUN Bypass Connected Successfully!")
	fmt.Printf("  • Profile:    %s%s%s\n", ui.ColorBold, activeProf.Name, ui.ColorReset)
	fmt.Printf("  • Server:     %s:%d\n", activeProf.Server, activeProf.Port)
	fmt.Printf("  • SNI:        %s\n", activeProf.SNI)
	fmt.Printf("  • Interface:  tun0 (Full OS Routed)\n\n")
	ui.Info("Background mode active. You can safely close this terminal window.")
	ui.Info("Run '%ssudo nuxmod%s' at any time to open the live dashboard.", ui.ColorCyan, ui.ColorReset)
}

func handleDisconnect() {
	if !daemon.IsDaemonRunning() {
		ui.Info("VPN is not currently running.")
		return
	}

	ui.Info("Sending disconnect signal to background daemon...")
	_ = daemon.StopBackgroundDaemon()
	ui.Success("VPN disconnected. System network routes and IPv6 cleanly restored.")
}

func handleStatus() {
	resp, err := daemon.SendIPCCommand(daemon.IPCRequest{Command: "STATUS"}, 1*time.Second)
	if err != nil || resp == nil || resp.Status == nil || !resp.Status.Running {
		fmt.Println("\033[31m○ Status: DISCONNECTED\033[0m")
		fmt.Println("Run 'sudo nuxmod connect' to start the VPN.")
		return
	}

	s := resp.Status
	fmt.Printf("\033[32m● Status: CONNECTED\033[0m\n")
	fmt.Printf("  • Active Node: %s\n", s.ProfileName)
	fmt.Printf("  • Server:      %s:%d (SNI: %s)\n", s.Server, s.Port, s.SNI)
	fmt.Printf("  • Latency:     %d ms\n", s.LatencyMs)
	fmt.Printf("  • Download:    %.1f KB/s (Total: %.1f MB)\n", s.RxSpeed/1024, float64(s.TotalRx)/(1024*1024))
	fmt.Printf("  • Upload:      %.1f KB/s (Total: %.1f MB)\n", s.TxSpeed/1024, float64(s.TotalTx)/(1024*1024))
	fmt.Println("\nRun 'sudo nuxmod' for full live dashboard.")
}

func handleLogs() {
	resp, err := daemon.SendIPCCommand(daemon.IPCRequest{Command: "LOGS"}, 2*time.Second)
	if err != nil || resp == nil || len(resp.Logs) == 0 {
		fmt.Println("No logs available (VPN may not be running).")
		return
	}
	fmt.Println("\033[36m=== Recent NuxMod Connection Logs ===\033[0m")
	for _, l := range resp.Logs {
		fmt.Println(l)
	}
}

func handleProfileCmd(profMgr *profile.Manager, args []string) {
	if len(args) == 0 || args[0] == "list" {
		profiles := profMgr.List()
		if len(profiles) == 0 {
			fmt.Println("No saved profiles. Add one with: nuxmod profile add \"vless://...\"")
			return
		}
		ui.Header("Saved VLESS Profiles")
		for i, p := range profiles {
			marker := " "
			tag := ""
			if p.Active {
				marker = "*"
				tag = ui.ColorGreen + " [ACTIVE]" + ui.ColorReset
			}
			fmt.Printf("  [%s] %d. %-24s (%s:%d)%s\n", marker, i+1, p.Name, p.Server, p.Port, tag)
		}
		fmt.Println()
		return
	}

	sub := args[0]
	switch sub {
	case "add":
		if len(args) < 2 {
			ui.Error("Usage: nuxmod profile add <vless_url> [optional_name]")
			return
		}
		name := ""
		if len(args) > 2 {
			name = args[2]
		}
		p, err := profMgr.Add(args[1], name)
		if err != nil {
			ui.Error("Failed: %v", err)
			return
		}
		ui.Success("Profile [%s] added successfully!", p.Name)

	case "delete", "rm":
		if len(args) < 2 {
			ui.Error("Usage: nuxmod profile delete <name|number>")
			return
		}
		target := args[1]
		profiles := profMgr.List()
		for i, p := range profiles {
			if fmt.Sprintf("%d", i+1) == target {
				target = p.ID
				break
			}
		}
		if err := profMgr.Delete(target); err != nil {
			ui.Error("Delete failed: %v", err)
		} else {
			ui.Success("Profile deleted.")
		}

	case "use", "select", "switch":
		if len(args) < 2 {
			ui.Error("Usage: nuxmod profile use <name|number>")
			return
		}
		target := args[1]
		profiles := profMgr.List()
		for i, p := range profiles {
			if fmt.Sprintf("%d", i+1) == target {
				target = p.ID
				break
			}
		}
		p, err := profMgr.SetActive(target)
		if err != nil {
			ui.Error("Failed: %v", err)
		} else {
			ui.Success("Active profile set to [%s].", p.Name)
			if daemon.IsDaemonRunning() {
				ui.Info("Reconnecting VPN with new profile...")
				handleDisconnect()
				handleConnect(profMgr, nil)
			}
		}
	}
}

func handleDaemon(profMgr *profile.Manager) {
	activeProf, err := profMgr.GetActive()
	if err != nil {
		os.Exit(1)
	}

	d := daemon.NewDaemon(profMgr)
	ctx := context.Background()
	_ = d.Run(ctx, activeProf)
}

func handleParse(args []string) {
	if len(args) == 0 {
		ui.Error("Missing VLESS link argument")
		return
	}
	vlessCfg, err := parser.ParseVlessLink(args[0])
	if err != nil {
		ui.Error("Parse failed: %v", err)
		return
	}
	ui.Header("VLESS Node Information")
	fmt.Printf("  Remark:          %s\n", vlessCfg.Remarks)
	fmt.Printf("  Server Address:  %s:%d\n", vlessCfg.Server, vlessCfg.Port)
	fmt.Printf("  UUID:            %s\n", vlessCfg.UUID)
	fmt.Printf("  Transport:       %s\n", vlessCfg.Network)
	fmt.Printf("  Security:        %s\n", vlessCfg.Security)
	fmt.Printf("  SNI Spoofing:    %s\n", vlessCfg.SNI)
	fmt.Printf("  uTLS:            %s\n", vlessCfg.Fingerprint)
	fmt.Println()
}

func handleUpdate() {
	ui.PrintBanner()
	ui.Header("NuxMod Software Self-Updater")
	fmt.Printf("Current Version: v%s\n", updater.CurrentVersion)
	fmt.Println("Checking GitHub for the latest release...")

	info, err := updater.CheckForUpdate()
	if err != nil {
		ui.Error("Failed to check for updates: %v", err)
		os.Exit(1)
	}

	if !info.HasUpdate {
		ui.Success("You are already using the latest version (v%s)!", info.LatestVersion)
		fmt.Print("Do you want to force re-download and reinstall anyway? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	} else {
		ui.Info("A new version is available: v%s (Current: v%s)", info.LatestVersion, info.CurrentVersion)
		if info.ReleaseNotes != "" {
			fmt.Printf("\nRelease Notes:\n%s\n\n", info.ReleaseNotes)
		}
		fmt.Print("Proceed with upgrade? [Y/n]: ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "n" || answer == "no" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	ui.Info("Downloading binary from: %s", info.DownloadURL)
	if err := updater.DownloadAndInstall(info.DownloadURL); err != nil {
		ui.Error("Update failed: %v", err)
		os.Exit(1)
	}

	ui.Success("Successfully upgraded NuxMod to v%s!", info.LatestVersion)
	fmt.Println("You can now run 'sudo nuxmod' to launch the updated tool.")
}
