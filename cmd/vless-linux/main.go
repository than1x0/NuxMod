package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/config"
	"github.com/Sathira-Laksahan/vless-linux/internal/engine"
	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
	"github.com/Sathira-Laksahan/vless-linux/internal/routing"
	"github.com/Sathira-Laksahan/vless-linux/internal/ui"
)

const AppVersion = "1.0.0"

func getAppDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	appDir := filepath.Join(home, ".vless-linux")
	_ = os.MkdirAll(appDir, 0755)
	return appDir
}

func main() {
	ui.PrintBanner()

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "connect":
		handleConnect(os.Args[2:])
	case "parse":
		handleParse(os.Args[2:])
	case "export":
		handleExport(os.Args[2:])
	case "test":
		handleTest(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("vless-linux version %s\n", AppVersion)
	default:
		// If user directly passed a vless:// link as the first argument
		if len(command) > 8 && command[:8] == "vless://" {
			handleConnect([]string{command})
		} else {
			ui.Error("Unknown command: %s", command)
			printUsage()
		}
	}
}

func printUsage() {
	ui.Header("Usage & Commands")
	fmt.Println("  vless-linux connect [vless_link]   Start TUN mode and route entire OS through VLESS")
	fmt.Println("  vless-linux parse   <vless_link>   Inspect and parse VLESS link parameters")
	fmt.Println("  vless-linux export  <vless_link>   Generate sing-box JSON configuration file")
	fmt.Println("  vless-linux test    <vless_link>   Test server latency and connectivity")
	fmt.Println("  vless-linux version                Show tool version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sudo ./vless-linux connect \"vless://...\"")
	fmt.Println("  sudo ./vless-linux connect                 (uses previously saved link)")
	fmt.Println()
}

func getLink(args []string) (string, error) {
	appDir := getAppDir()
	savedLinkFile := filepath.Join(appDir, "last_used.txt")

	if len(args) > 0 && args[0] != "" {
		link := args[0]
		_ = os.WriteFile(savedLinkFile, []byte(link), 0600)
		return link, nil
	}

	// Try reading saved link
	data, err := os.ReadFile(savedLinkFile)
	if err == nil && len(data) > 0 {
		ui.Info("Using saved VLESS link from previous session")
		return string(data), nil
	}

	return "", fmt.Errorf("no VLESS link provided and no saved link found")
}

func handleConnect(args []string) {
	link, err := getLink(args)
	if err != nil {
		ui.Error("%v", err)
		ui.Info("Usage: sudo ./vless-linux connect \"vless://...\"")
		os.Exit(1)
	}

	ui.Info("Parsing VLESS link...")
	vlessCfg, err := parser.ParseVlessLink(link)
	if err != nil {
		ui.Error("Failed to parse VLESS link: %v", err)
		os.Exit(1)
	}

	displayNodeInfo(vlessCfg)

	// Environment checks
	if err := routing.CheckLinuxEnvironment(); err != nil {
		ui.Warn("%v", err)
		ui.Warn("TUN interface creation requires Root (sudo). If connection fails, please run with 'sudo'.")
	}

	// Test server latency
	ui.Info("Testing latency to server %s:%d...", vlessCfg.Server, vlessCfg.Port)
	latency, err := routing.TestLatency(vlessCfg.Server, vlessCfg.Port, 3*time.Second)
	if err != nil {
		ui.Warn("Server ping failed: %v (ISP might be blocking direct IP, proceed with SNI spoofing)", err)
	} else {
		ui.Success("Server reachable! Direct TCP Latency: %v", latency)
	}

	// Generate Sing-box TUN config
	appDir := getAppDir()
	configPath := filepath.Join(appDir, "config.json")
	singboxCfg, err := config.GenerateSingBoxConfig(vlessCfg)
	if err != nil {
		ui.Error("Failed to generate configuration: %v", err)
		os.Exit(1)
	}

	if err := config.SaveConfigToFile(singboxCfg, configPath); err != nil {
		ui.Error("Failed to save configuration: %v", err)
		os.Exit(1)
	}
	ui.Success("Generated TUN configuration saved to %s", configPath)

	// Ensure sing-box core binary is present
	mgr := engine.NewEngineManager(configPath)
	binDir := filepath.Join(appDir, "bin")
	binPath, err := mgr.EnsureBinary(binDir)
	if err != nil {
		ui.Error("Sing-box core binary error: %v", err)
		os.Exit(1)
	}
	ui.Success("Using Sing-box core: %s", binPath)

	// Start Engine
	ui.Header("Starting System-Wide TUN Bypass")
	ui.Info("Creating Virtual TUN device 'tun0'...")
	ui.Info("Setting default routes for Web, Terminal, and Background apps...")
	ui.Info("Press Ctrl+C at any time to disconnect and restore original network.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Intercept Ctrl+C / SIGINT / SIGTERM for safe cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		ui.Warn("\nDisconnect signal received! Cleaning up network routes and TUN interface...")
		_ = mgr.Stop()
		cancel()
		ui.Success("Network routes restored cleanly. Goodbye!")
		os.Exit(0)
	}()

	if err := mgr.Start(ctx, os.Stdout, os.Stderr); err != nil {
		ui.Error("Failed to start bypass engine: %v", err)
		os.Exit(1)
	}

	// Wait for process to exit
	if err := mgr.Wait(); err != nil {
		ui.Error("Engine exited with error: %v", err)
	}
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
	displayNodeInfo(vlessCfg)
}

func handleExport(args []string) {
	if len(args) == 0 {
		ui.Error("Usage: vless-linux export <vless_link> [output_file.json]")
		return
	}
	link := args[0]
	outFile := "sing-box-config.json"
	if len(args) > 1 {
		outFile = args[1]
	}

	vlessCfg, err := parser.ParseVlessLink(link)
	if err != nil {
		ui.Error("Parse failed: %v", err)
		return
	}

	singboxCfg, err := config.GenerateSingBoxConfig(vlessCfg)
	if err != nil {
		ui.Error("Failed to generate configuration: %v", err)
		return
	}

	if err := config.SaveConfigToFile(singboxCfg, outFile); err != nil {
		ui.Error("Save failed: %v", err)
		return
	}
	ui.Success("Configuration successfully exported to: %s", outFile)
}

func handleTest(args []string) {
	link, err := getLink(args)
	if err != nil {
		ui.Error("%v", err)
		return
	}
	vlessCfg, err := parser.ParseVlessLink(link)
	if err != nil {
		ui.Error("Parse failed: %v", err)
		return
	}

	displayNodeInfo(vlessCfg)
	ui.Info("Testing TCP dial to %s:%d...", vlessCfg.Server, vlessCfg.Port)
	latency, err := routing.TestLatency(vlessCfg.Server, vlessCfg.Port, 5*time.Second)
	if err != nil {
		ui.Error("Connection test failed: %v", err)
	} else {
		ui.Success("Connection successful! Latency: %v", latency)
	}
}

func displayNodeInfo(cfg *parser.VlessConfig) {
	ui.Header("VLESS Node Information")
	fmt.Printf("  Remark / Tag:    %s%s%s\n", ui.ColorBold, cfg.Remarks, ui.ColorReset)
	fmt.Printf("  Server Address:  %s:%d\n", cfg.Server, cfg.Port)
	fmt.Printf("  UUID:            %s\n", cfg.UUID)
	fmt.Printf("  Transport:       %s\n", cfg.Network)
	fmt.Printf("  Security:        %s\n", cfg.Security)
	fmt.Printf("  SNI Spoofing:    %s%s%s\n", ui.ColorGreen, cfg.SNI, ui.ColorReset)
	fmt.Printf("  uTLS Fingerprint:%s%s%s\n", ui.ColorGreen, cfg.Fingerprint, ui.ColorReset)
	if cfg.Insecure {
		fmt.Printf("  Allow Insecure:  %strue (required for SNI spoofing)%s\n", ui.ColorYellow, ui.ColorReset)
	}
	if cfg.Flow != "" {
		fmt.Printf("  Flow:            %s\n", cfg.Flow)
	}
	if cfg.Path != "" {
		fmt.Printf("  Path:            %s\n", cfg.Path)
	}
	fmt.Println()
}

func init() {
	flag.Usage = printUsage
}
