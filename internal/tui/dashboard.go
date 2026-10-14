package tui

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Sathira-Laksahan/vless-linux/internal/daemon"
	"github.com/Sathira-Laksahan/vless-linux/internal/profile"
)

// Bluloco / Tokyo Night 24-bit TrueColor Palette (Matching Reference Screenshot)
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Italic = "\033[3m"

	BorderColor = "\033[38;2;71;85;105m"   // Sleek Slate #475569
	TagBracket  = "\033[38;2;100;116;139m" // Muted Gray Brackets #64748b
	AccentCoral = "\033[38;2;244;63;94m"   // Coral/Red #f43f5e (Hotkeys & Headers)
	AccentCyan  = "\033[38;2;56;189;248m"  // Electric Cyan #38bdf8
	AccentBlue  = "\033[38;2;96;165;250m"  // Sky Blue #60a5fa
	AccentGreen = "\033[38;2;52;211;153m"  // Emerald Green #34d399
	AccentAmber = "\033[38;2;251;191;36m"  // Amber / Gold #fbbf24
	AccentRose  = "\033[38;2;251;113;133m" // Soft Rose #fb7185
	TextMain    = "\033[38;2;241;245;249m" // Crisp Off-White #f1f5f9
	TextMuted   = "\033[38;2;148;163;184m" // Slate Muted #94a3b8
	BgHighlight = "\033[48;2;30;41;59m"    // Slate Dark Background for Badges

	GradClock0 = "\033[38;2;125;211;252m" // Light Cyan #7dd3fc
	GradClock1 = "\033[38;2;56;189;248m"  // Electric Cyan #38bdf8
	GradClock2 = "\033[38;2;37;99;235m"   // Vibrant Royal Blue #2563eb

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

var (
	rxHistory []float64
	peakRx    float64
	peakTx    float64
)

// Digits 0-9 and : for 3-row Digital Block Clock
var digitRows = map[rune][3]string{
	'0': {"███", "█ █", "███"},
	'1': {" ██", " ██", " ██"},
	'2': {"███", " ▄█", "███"},
	'3': {"███", "▄██", "███"},
	'4': {"█ █", "███", "  █"},
	'5': {"███", "██▄", "███"},
	'6': {"███", "█▄█", "███"},
	'7': {"███", "  █", "  █"},
	'8': {"███", "█▄█", "███"},
	'9': {"███", "▀██", "███"},
	':': {"   ", " ▄ ", " ▄ "},
	' ': {"   ", "   ", "   "},
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalSize dynamically retrieves the actual columns and lines of the terminal.
func getTerminalSize() (width, height int) {
	ws := &winsize{}

	// 1. Try /dev/tty first (the real controlling terminal)
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		ret, _, _ := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(0x5413), uintptr(unsafe.Pointer(ws)))
		f.Close()
		if int(ret) == 0 && ws.Col > 0 && ws.Row > 0 {
			return int(ws.Col), int(ws.Row)
		}
	}

	// 2. Try stdout, stdin, stderr
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stdin.Fd(), os.Stderr.Fd()} {
		ret, _, _ := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(0x5413), uintptr(unsafe.Pointer(ws)))
		if int(ret) == 0 && ws.Col > 0 && ws.Row > 0 {
			return int(ws.Col), int(ws.Row)
		}
	}

	// 3. Fallback to stty command
	if out, err := exec.Command("stty", "size").Output(); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(out)))
		if len(fields) == 2 {
			r, err1 := strconv.Atoi(fields[0])
			c, err2 := strconv.Atoi(fields[1])
			if err1 == nil && err2 == nil && c > 0 && r > 0 {
				return c, r
			}
		}
	}

	// 4. Fallback to env COLUMNS
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if c, err := strconv.Atoi(cols); err == nil && c > 0 {
			lines := 30
			if l := os.Getenv("LINES"); l != "" {
				if parsedL, err := strconv.Atoi(l); err == nil && parsedL > 0 {
					lines = parsedL
				}
			}
			return c, lines
		}
	}

	return 106, 30
}

// RunDashboard runs the interactive terminal dashboard with modern bluloco aesthetic.
func RunDashboard(profMgr *profile.Manager) {
	// Enter Alternate Screen Buffer (prevents scrolling the terminal history)
	fmt.Print("\033[?1049h\033[H\033[2J")

	var exitMsg string
	defer func() {
		// Exit Alternate Screen Buffer and restore normal terminal
		fmt.Print("\033[?1049l\033[?25h")
		if exitMsg != "" {
			fmt.Print(exitMsg)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	currentMode := modeDashboard

	inputChan := make(chan string)
	go func() {
		for scanner.Scan() {
			inputChan <- strings.TrimSpace(scanner.Text())
		}
		close(inputChan)
	}()

	ticker := time.NewTicker(1000 * time.Millisecond)
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
					exitMsg = "\n" + AccentAmber + "[*] Dashboard detached. VPN is running uninterrupted in background!\n" +
						AccentCyan + "[*] Run 'sudo nuxmod' at any time to reopen this dashboard.\n\n" + Reset
					return

				case "x":
					_ = daemon.StopBackgroundDaemon()
					exitMsg = "\n" + AccentRose + "[*] Disconnecting VPN and restoring original network routing...\n" +
						AccentGreen + "[+] Network cleanly restored. Goodbye!\n\n" + Reset
					return

				default:
					renderDashboard(profMgr)
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
				currentMode = modeDashboard
				renderDashboard(profMgr)
			}

		case <-ticker.C:
			if currentMode == modeDashboard {
				renderDashboard(profMgr)
			}
		}
	}
}

func renderDashboard(profMgr *profile.Manager) {
	// Dynamically detect terminal width
	termWidth, _ := getTerminalSize()
	if termWidth < 80 {
		termWidth = 80
	}

	// Calculate panel widths to fill 100% of the terminal width
	wLeft := (termWidth - 2) / 2
	wRight := termWidth - 2 - wLeft

	// Fetch daemon status over IPC
	resp, _ := daemon.SendIPCCommand(daemon.IPCRequest{Command: "STATUS"}, 400*time.Millisecond)
	var status *daemon.DaemonStatus
	if resp != nil && resp.Success {
		status = resp.Status
	}

	activeProf, _ := profMgr.GetActive()

	// Update traffic wave history (keep up to chartWidth data points)
	chartWidth := wRight - 12
	if chartWidth < 30 {
		chartWidth = 30
	}
	if chartWidth > 120 {
		chartWidth = 120
	}

	if status != nil && status.Running {
		rxHistory = append(rxHistory, status.RxSpeed)
		if len(rxHistory) > chartWidth {
			rxHistory = rxHistory[len(rxHistory)-chartWidth:]
		}
		if status.RxSpeed > peakRx {
			peakRx = status.RxSpeed
		}
		if status.TxSpeed > peakTx {
			peakTx = status.TxSpeed
		}
	} else {
		rxHistory = append(rxHistory, 0)
		if len(rxHistory) > chartWidth {
			rxHistory = rxHistory[len(rxHistory)-chartWidth:]
		}
	}

	now := time.Now()

	// 1. TOP PANELS (Left: Clock & Connection, Right: Profiles & Routing)
	leftTop := renderClockStatusPanel(status, activeProf, now, wLeft)
	rightTop := renderProfilesRoutingPanel(profMgr, status, wRight)

	// 2. BOTTOM PANELS (Left: Resources & TUN, Right: Traffic Wave & Events)
	leftBottom := renderResourcesPanel(status, wLeft)
	rightBottom := renderTrafficWavePanel(status, wRight)

	footer := buildFooterBar(status, now, termWidth)

	// Assemble frame into a single buffer to guarantee zero flicker and zero scroll!
	var buf strings.Builder
	// Move cursor to top-left (1,1) in alternate buffer
	buf.WriteString("\033[H")

	// Print Top Panels side by side
	for i := 0; i < len(leftTop) && i < len(rightTop); i++ {
		buf.WriteString(leftTop[i])
		buf.WriteString("  ")
		buf.WriteString(rightTop[i])
		buf.WriteString("\033[K\n")
	}
	buf.WriteString("\033[K\n")

	// Print Bottom Panels side by side
	for i := 0; i < len(leftBottom) && i < len(rightBottom); i++ {
		buf.WriteString(leftBottom[i])
		buf.WriteString("  ")
		buf.WriteString(rightBottom[i])
		buf.WriteString("\033[K\n")
	}
	buf.WriteString("\033[K\n")

	// Print Status Footer
	buf.WriteString(footer)
	buf.WriteString("\033[K\n\n")

	// Prompt
	buf.WriteString(TextMuted)
	buf.WriteString("❯ ")
	buf.WriteString(Reset)
	buf.WriteString("\033[K")

	// Clear any leftover lines below this frame
	buf.WriteString("\033[J")

	// Write entire frame atomically
	_, _ = os.Stdout.WriteString(buf.String())
}

// Top-Left Panel: Clock, Date & Connection Status
func renderClockStatusPanel(status *daemon.DaemonStatus, activeProf *profile.Profile, now time.Time, w int) []string {
	lines := make([]string, 11)

	// Header: ╭─ [C]lock ─ Connection ───────────────────────────╮
	dashCount := w - 25
	if dashCount < 2 {
		dashCount = 2
	}
	lines[0] = BorderColor + "╭─ " + TagBracket + "[" + AccentCoral + Bold + "C" + Reset + TagBracket + "]lock " + TextMuted + "─ Connection " + BorderColor + strings.Repeat("─", dashCount) + "╮" + Reset

	// Digital Clock (Centered)
	clockRows := renderClockRows(now)
	clockPadLen := (w - 4 - 19) / 2
	if clockPadLen < 0 {
		clockPadLen = 0
	}
	clockPad := strings.Repeat(" ", clockPadLen)
	lines[1] = wrapBoxLine(clockPad+clockRows[0], w)
	lines[2] = wrapBoxLine(clockPad+clockRows[1], w)
	lines[3] = wrapBoxLine(clockPad+clockRows[2], w)

	// Time & Date Subtitle (Centered)
	timeSub := now.Format("15:04:05 · Mon Jan 02, 2006")
	timePadLen := (w - 4 - len([]rune(timeSub))) / 2
	if timePadLen < 0 {
		timePadLen = 0
	}
	timePad := strings.Repeat(" ", timePadLen)
	lines[4] = wrapBoxLine(timePad+TextMuted+timeSub+Reset, w)

	// Section Divider (Full width)
	divLen := (w - 4 - 18) / 2
	if divLen < 2 {
		divLen = 2
	}
	divRight := w - 4 - 18 - divLen
	if divRight < 2 {
		divRight = 2
	}
	lines[5] = wrapBoxLine(BorderColor+strings.Repeat("─", divLen)+TextMuted+" Network Status "+BorderColor+strings.Repeat("─", divRight)+Reset, w)

	if status != nil && status.Running {
		latStr := fmt.Sprintf("%dms", status.LatencyMs)
		if status.LatencyMs <= 0 {
			latStr = "testing..."
		}
		uptimeStr := formatDuration(time.Duration(status.UptimeSeconds) * time.Second)

		lines[6] = wrapBoxLine(TextMuted+"State   : "+AccentGreen+Bold+"● CONNECTED"+Reset+TextMuted+" (tun0 / gVisor)"+Reset, w)
		lines[7] = wrapBoxLine(TextMuted+"Node    : "+AccentAmber+Bold+truncate(status.ProfileName, w-24)+Reset+TextMuted+" ("+truncate(status.SNI, 18)+")"+Reset, w)
		lines[8] = wrapBoxLine(TextMuted+"Ping    : "+AccentGreen+latStr+Reset+TextMuted+"  Uptime: "+TextMain+uptimeStr+Reset, w)
	} else {
		nodeName := "None"
		sniName := "none"
		if activeProf != nil {
			nodeName = activeProf.Name
			sniName = activeProf.SNI
		}
		lines[6] = wrapBoxLine(TextMuted+"State   : "+AccentRose+Bold+"○ DISCONNECTED"+Reset+TextMuted+" (ISP Direct)"+Reset, w)
		lines[7] = wrapBoxLine(TextMuted+"Node    : "+AccentAmber+truncate(nodeName, w-24)+Reset+TextMuted+" ("+truncate(sniName, 18)+")"+Reset, w)
		lines[8] = wrapBoxLine(TextMuted+"Ping    : "+TextMuted+"offline"+Reset+TextMuted+"  Action: "+AccentGreen+"[c] to connect"+Reset, w)
	}

	lines[9] = wrapBoxLine(TagBracket+"["+AccentCoral+"c"+TagBracket+"] "+TextMuted+"toggle  "+TagBracket+"["+AccentCoral+"p"+TagBracket+"] "+TextMuted+"switch  "+TagBracket+"["+AccentCoral+"r"+TagBracket+"] "+TextMuted+"ping"+Reset, w)
	lines[10] = BorderColor + "╰" + strings.Repeat("─", w-2) + "╯" + Reset
	return lines
}

// Top-Right Panel: Profiles & System Routing
func renderProfilesRoutingPanel(profMgr *profile.Manager, status *daemon.DaemonStatus, w int) []string {
	lines := make([]string, 11)

	headerRight := TextMuted + "[nuxmod]"
	dashCount := w - 23 - len([]rune(stripANSI(headerRight))) - 1
	if dashCount < 2 {
		dashCount = 2
	}
	lines[0] = BorderColor + "╭─ " + TagBracket + "[" + AccentCoral + Bold + "P" + Reset + TagBracket + "]rofiles " + TextMuted + "─ Routing " + BorderColor + strings.Repeat("─", dashCount) + headerRight + BorderColor + "─╮" + Reset

	// Responsive Table columns based on width
	nameW := 14
	serverW := 18
	sniW := 14
	if w >= 80 {
		nameW = (w - 46) / 2
		serverW = (w - 46) / 2
		sniW = 16
		if nameW > 26 {
			nameW = 26
		}
		if serverW > 32 {
			serverW = 32
		}
	}

	if w >= 75 {
		lines[1] = wrapBoxLine(TextMuted+Bold+fmt.Sprintf(" #  %-*s %-*s %-*s PING", nameW, "NAME", serverW, "SERVER", sniW, "SNI")+Reset, w)
	} else {
		lines[1] = wrapBoxLine(TextMuted+Bold+" #  NAME             SERVER            PING"+Reset, w)
	}

	profiles := profMgr.List()
	if len(profiles) == 1 {
		p := profiles[0]
		isActive := (status != nil && status.ProfileName == p.Name) || (status == nil && p.Active)
		marker := TextMuted + " ○" + Reset
		badge := ""
		nameColor := TextMain
		pingStr := "..."
		if isActive {
			marker = AccentGreen + Bold + " ●" + Reset
			badge = " " + BgHighlight + AccentGreen + Bold + "ACTIVE" + Reset
			nameColor = AccentAmber + Bold
			if status != nil && status.Running && status.LatencyMs > 0 {
				pingStr = fmt.Sprintf("%dms", status.LatencyMs)
			}
		}

		var lineContent string
		if w >= 75 {
			lineContent = fmt.Sprintf("%s 1. %-*s %-*s %-*s %s%s",
				marker,
				nameW, nameColor+truncate(p.Name, nameW)+Reset,
				serverW, TextMuted+truncate(fmt.Sprintf("%s:%d", p.Server, p.Port), serverW)+Reset,
				sniW, AccentCyan+truncate(p.SNI, sniW)+Reset,
				AccentCyan+padRight(pingStr, 6)+Reset, badge)
		} else {
			lineContent = fmt.Sprintf("%s 1. %s  %s  %s%s",
				marker, nameColor+padRight(truncate(p.Name, 14), 14)+Reset,
				TextMuted+padRight(truncate(p.Server, 16), 16)+Reset, AccentCyan+padRight(pingStr, 6)+Reset, badge)
		}
		lines[2] = wrapBoxLine(lineContent, w)
		lines[3] = wrapBoxLine(TextMuted+"   Endpoint  : "+TextMain+truncate(fmt.Sprintf("%s:%d", p.Server, p.Port), w-18)+Reset, w)
		lines[4] = wrapBoxLine(TextMuted+"   TLS SNI   : "+AccentCyan+truncate(p.SNI, w-18)+Reset+TextMuted+" (Security TLS Handshake)"+Reset, w)
	} else {
		slotIndex := 2
		for i := 0; i < 3 && i < len(profiles); i++ {
			p := profiles[i]
			isActive := (status != nil && status.ProfileName == p.Name) || (status == nil && p.Active)
			marker := TextMuted + " ○" + Reset
			badge := ""
			nameColor := TextMain
			pingStr := "..."

			if isActive {
				marker = AccentGreen + Bold + " ●" + Reset
				badge = " " + BgHighlight + AccentGreen + Bold + "ACTIVE" + Reset
				nameColor = AccentAmber + Bold
				if status != nil && status.Running && status.LatencyMs > 0 {
					pingStr = fmt.Sprintf("%dms", status.LatencyMs)
				}
			}

			var lineContent string
			if w >= 75 {
				lineContent = fmt.Sprintf("%s %d. %-*s %-*s %-*s %s%s",
					marker, i+1,
					nameW, nameColor+truncate(p.Name, nameW)+Reset,
					serverW, TextMuted+truncate(fmt.Sprintf("%s:%d", p.Server, p.Port), serverW)+Reset,
					sniW, AccentCyan+truncate(p.SNI, sniW)+Reset,
					AccentCyan+padRight(pingStr, 6)+Reset, badge)
			} else {
				lineContent = fmt.Sprintf("%s %d. %s  %s  %s%s",
					marker, i+1, nameColor+padRight(truncate(p.Name, 14), 14)+Reset,
					TextMuted+padRight(truncate(p.Server, 16), 16)+Reset, AccentCyan+padRight(pingStr, 6)+Reset, badge)
			}
			lines[slotIndex] = wrapBoxLine(lineContent, w)
			slotIndex++
		}

		for slotIndex < 5 {
			if len(profiles) == 0 && slotIndex == 2 {
				lines[slotIndex] = wrapBoxLine(TextMuted+"[No profiles found. Press 'a' to add a link]"+Reset, w)
			} else {
				lines[slotIndex] = wrapBoxLine("", w)
			}
			slotIndex++
		}
	}

	lines[5] = wrapBoxLine(BorderColor+strings.Repeat("─", w-4)+Reset, w)
	lines[6] = wrapBoxLine(TextMuted+"Routing : "+AccentCyan+"Full OS TUN Bypass (tun0 -> 172.19.0.1/30 gVisor)"+Reset, w)
	lines[7] = wrapBoxLine(TextMuted+"DNS     : "+AccentCyan+"1.1.1.1 (TLS Encrypted Remote Upstream Resolver)"+Reset, w)
	lines[8] = wrapBoxLine(TextMuted+"Bypass  : "+TextMuted+"LAN Direct (192.168.0.0/16, 10.0.0.0/8, 127.0.0.0/8)"+Reset, w)
	lines[9] = wrapBoxLine(TagBracket+"["+AccentCoral+"a"+TagBracket+"] "+TextMuted+"add link  "+TagBracket+"["+AccentCoral+"d"+TagBracket+"] "+TextMuted+"delete  "+TagBracket+"["+AccentCoral+"p"+TagBracket+"] "+TextMuted+"select"+Reset, w)
	lines[10] = BorderColor + "╰" + strings.Repeat("─", w-2) + "╯" + Reset
	return lines
}

// Bottom-Left Panel: Resources & TUN Network Adapter
func renderResourcesPanel(status *daemon.DaemonStatus, w int) []string {
	lines := make([]string, 11)

	headerRight := TextMuted + "up 01h 42m · tun0"
	if status != nil && status.Running {
		headerRight = TextMuted + "up " + formatDuration(time.Duration(status.UptimeSeconds)*time.Second) + " · tun0"
	}
	dashCount := w - 19 - len([]rune(stripANSI(headerRight))) - 1
	if dashCount < 2 {
		dashCount = 2
	}
	lines[0] = BorderColor + "╭─ " + TagBracket + "[" + AccentCoral + Bold + "R" + Reset + TagBracket + "]esources " + TextMuted + "─ TUN " + BorderColor + strings.Repeat("─", dashCount) + headerRight + BorderColor + "─╮" + Reset

	lines[1] = wrapBoxLine(TextMuted+Bold+"Bandwidth Throughput & Realtime Rates:"+Reset, w)

	barW := w - 42
	if barW < 12 {
		barW = 12
	}
	if barW > 45 {
		barW = 45
	}

	if status != nil && status.Running {
		lines[2] = wrapBoxLine(TextMuted+"DL : "+AccentGreen+Bold+padRight(formatBytes(status.RxSpeed)+"/s", 11)+Reset+renderBar(status.RxSpeed, 6*1024*1024, barW)+TextMuted+" Pk: "+TextMain+formatBytes(peakRx)+"/s"+Reset, w)
		lines[3] = wrapBoxLine(TextMuted+"UL : "+AccentBlue+Bold+padRight(formatBytes(status.TxSpeed)+"/s", 11)+Reset+renderBar(status.TxSpeed, 2*1024*1024, barW)+TextMuted+" Pk: "+TextMain+formatBytes(peakTx)+"/s"+Reset, w)
		lines[4] = wrapBoxLine(BorderColor+strings.Repeat("─", w-4)+Reset, w)
		lines[5] = wrapBoxLine(TextMuted+"Data In  : "+AccentGreen+"⬇ "+padRight(formatBytes(float64(status.TotalRx)), 10)+Reset+TextMuted+"  Peak In  : "+TextMain+"⬇ "+formatBytes(peakRx)+"/s"+Reset, w)
		lines[6] = wrapBoxLine(TextMuted+"Data Out : "+AccentBlue+"⬆ "+padRight(formatBytes(float64(status.TotalTx)), 10)+Reset+TextMuted+"  Peak Out : "+TextMain+"⬆ "+formatBytes(peakTx)+"/s"+Reset, w)
	} else {
		lines[2] = wrapBoxLine(TextMuted+"DL : 0 B/s      "+strings.Repeat("░", barW)+" Pk: 0 B/s"+Reset, w)
		lines[3] = wrapBoxLine(TextMuted+"UL : 0 B/s      "+strings.Repeat("░", barW)+" Pk: 0 B/s"+Reset, w)
		lines[4] = wrapBoxLine(BorderColor+strings.Repeat("─", w-4)+Reset, w)
		lines[5] = wrapBoxLine(TextMuted+"Data In  : ⬇ 0 B           Peak In  : ⬇ 0 B/s"+Reset, w)
		lines[6] = wrapBoxLine(TextMuted+"Data Out : ⬆ 0 B           Peak Out : ⬆ 0 B/s"+Reset, w)
	}

	lines[7] = wrapBoxLine(TextMuted+"Adapter  : "+AccentCyan+"tun0"+Reset+TextMuted+" (172.19.0.1/30, MTU 9000, gVisor Full Routing)"+Reset, w)
	lines[8] = wrapBoxLine(TextMuted+"Core     : "+TextMuted+"sing-box 1.8.14 (amd64-linux, active daemon)"+Reset, w)
	lines[9] = wrapBoxLine(TagBracket+"["+AccentCoral+"c"+TagBracket+"] "+TextMuted+"connect  "+TagBracket+"["+AccentCoral+"l"+TagBracket+"] "+TextMuted+"logs  "+TagBracket+"["+AccentCoral+"r"+TagBracket+"] "+TextMuted+"refresh"+Reset, w)
	lines[10] = BorderColor + "╰" + strings.Repeat("─", w-2) + "╯" + Reset
	return lines
}

// Bottom-Right Panel: Realtime Traffic Wave & Live Event Stream
func renderTrafficWavePanel(status *daemon.DaemonStatus, w int) []string {
	lines := make([]string, 11)

	headerRight := TextMuted + "Live Wave"
	dashCount := w - 24 - len([]rune(stripANSI(headerRight))) - 1
	if dashCount < 2 {
		dashCount = 2
	}
	lines[0] = BorderColor + "╭─ " + TagBracket + "[" + AccentCoral + Bold + "T" + Reset + TagBracket + "]raffic Wave " + TextMuted + "─ Events " + BorderColor + strings.Repeat("─", dashCount) + headerRight + BorderColor + "─╮" + Reset

	// Multi-Line Real-Time Traffic Chart
	chartW := w - 12
	if chartW < 25 {
		chartW = 25
	}
	if chartW > 120 {
		chartW = 120
	}
	chartLines := renderMultiLineChart(rxHistory, chartW, 4)
	lines[1] = wrapBoxLine(chartLines[0], w)
	lines[2] = wrapBoxLine(chartLines[1], w)
	lines[3] = wrapBoxLine(chartLines[2], w)
	lines[4] = wrapBoxLine(chartLines[3], w)
	lines[5] = wrapBoxLine(chartLines[4], w)

	lines[6] = wrapBoxLine(BorderColor+strings.Repeat("─", w-4)+Reset, w)

	// Live Event Stream
	log1 := "[Waiting for network traffic / Connection idle]"
	log2 := "[sing-box core ready / tun0 listening]"
	if status != nil && len(status.RecentLogs) > 0 {
		idx := len(status.RecentLogs) - 1
		log1 = status.RecentLogs[idx]
		if idx > 0 {
			log2 = status.RecentLogs[idx-1]
		}
	}

	maxLogLen := w - 6
	if maxLogLen < 30 {
		maxLogLen = 30
	}
	lines[7] = wrapBoxLine(AccentGreen+"● "+Reset+colorizeLog(truncate(log1, maxLogLen)), w)
	lines[8] = wrapBoxLine(AccentCyan+"● "+Reset+colorizeLog(truncate(log2, maxLogLen)), w)
	lines[9] = wrapBoxLine(TagBracket+"["+AccentCoral+"l"+TagBracket+"] "+TextMuted+"full logs  "+TagBracket+"["+AccentCoral+"q"+TagBracket+"] "+TextMuted+"detach  "+TagBracket+"["+AccentCoral+"x"+TagBracket+"] "+TextMuted+"quit"+Reset, w)
	lines[10] = BorderColor + "╰" + strings.Repeat("─", w-2) + "╯" + Reset
	return lines
}

func buildFooterBar(status *daemon.DaemonStatus, now time.Time, termWidth int) string {
	statusTag := AccentRose + "tun0:idle" + Reset
	if status != nil && status.Running {
		statusTag = AccentGreen + "tun0:connected" + Reset
	}

	leftPart := TextMuted + "nuxmod v2.0 " + BorderColor + "│" + Reset +
		" " + TextMuted + now.Format("15:04:05") + " " + BorderColor + "│" + Reset +
		" Focus: " + statusTag + " " + BorderColor + "│" + Reset +
		" Scheme: " + AccentCyan + "bluloco" + Reset + " " + BorderColor + "│" + Reset +
		" " + AccentCoral + "c" + Reset + TextMuted + ":toggle " +
		AccentCoral + "p" + Reset + TextMuted + ":switch " +
		AccentCoral + "a" + Reset + TextMuted + ":add " +
		AccentCoral + "d" + Reset + TextMuted + ":del " +
		AccentCoral + "l" + Reset + TextMuted + ":logs " +
		AccentGreen + "q" + Reset + TextMuted + ":detach " +
		AccentRose + "x" + Reset + TextMuted + ":exit" + Reset

	vis := len([]rune(stripANSI(leftPart)))
	if termWidth > vis {
		return leftPart + strings.Repeat(" ", termWidth-vis)
	}
	return leftPart
}

func handleConnectToggle(profMgr *profile.Manager) {
	if daemon.IsDaemonRunning() {
		fmt.Print("\n" + AccentAmber + "[*] Disconnecting VPN..." + Reset)
		_ = daemon.StopBackgroundDaemon()
		time.Sleep(500 * time.Millisecond)
	} else {
		activeProf, err := profMgr.GetActive()
		if err != nil {
			fmt.Print("\n" + AccentRose + "[-] No active profile! Press [a] to add one." + Reset + "\n")
			time.Sleep(1500 * time.Millisecond)
			return
		}
		fmt.Printf("\n"+AccentGreen+"[*] Spawning background daemon for [%s]..."+Reset+"\n", activeProf.Name)
		if err := daemon.StartBackgroundDaemon(); err != nil {
			fmt.Printf(AccentRose+"[-] Failed: %v"+Reset+"\n", err)
			time.Sleep(2 * time.Second)
			return
		}
		time.Sleep(600 * time.Millisecond)
	}
}

func promptAddProfile() {
	termWidth, _ := getTerminalSize()
	if termWidth < 80 {
		termWidth = 80
	}
	fmt.Print(ClearScreen)
	title := "Add New VLESS Profile Node"
	pad := (termWidth - 2 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Println(BorderColor + "╭" + strings.Repeat("─", termWidth-2) + "╮" + Reset)
	fmt.Println(BorderColor + "│" + Reset + strings.Repeat(" ", pad) + AccentCyan + Bold + title + Reset + strings.Repeat(" ", termWidth-2-len(title)-pad) + BorderColor + "│" + Reset)
	fmt.Println(BorderColor + "╰" + strings.Repeat("─", termWidth-2) + "╯" + Reset)
	fmt.Println(TextMuted + "Paste your complete VLESS link below (or type 'c' to cancel):" + Reset)
	fmt.Print(AccentCoral + "\nLink ❯ " + Reset)
}

func processAddProfile(profMgr *profile.Manager, link string, scanner *bufio.Scanner) {
	link = strings.TrimSpace(link)
	if link == "" || strings.ToLower(link) == "c" || strings.ToLower(link) == "cancel" {
		return
	}

	fmt.Print(TextMuted + "Profile Name (optional, Enter to auto-name) ❯ " + Reset)
	var name string
	if scanner.Scan() {
		name = strings.TrimSpace(scanner.Text())
	}

	prof, err := profMgr.Add(link, name)
	if err != nil {
		fmt.Printf("\n"+AccentRose+"[-] Failed to add profile: %v"+Reset+"\n", err)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Printf("\n"+AccentGreen+"[+] Profile [%s] added and saved successfully!"+Reset+"\n", prof.Name)
	time.Sleep(1 * time.Second)
}

func promptDeleteProfile(profMgr *profile.Manager) {
	termWidth, _ := getTerminalSize()
	if termWidth < 80 {
		termWidth = 80
	}
	fmt.Print(ClearScreen)
	title := "Delete Profile Node"
	pad := (termWidth - 2 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Println(BorderColor + "╭" + strings.Repeat("─", termWidth-2) + "╮" + Reset)
	fmt.Println(BorderColor + "│" + Reset + strings.Repeat(" ", pad) + AccentRose + Bold + title + Reset + strings.Repeat(" ", termWidth-2-len(title)-pad) + BorderColor + "│" + Reset)
	fmt.Println(BorderColor + "╰" + strings.Repeat("─", termWidth-2) + "╯" + Reset)
	profiles := profMgr.List()
	for i, p := range profiles {
		fmt.Printf("  %s%d.%s %-24s %s\n", AccentCoral, i+1, Reset, p.Name, TextMuted+p.Server+Reset)
	}
	fmt.Print(TextMuted + "\nEnter number to delete (or 'c' to cancel) ❯ " + Reset)
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
		fmt.Printf("\n"+AccentRose+"[-] Delete error: %v"+Reset+"\n", err)
		time.Sleep(2 * time.Second)
	} else {
		fmt.Println("\n" + AccentGreen + "[+] Profile removed." + Reset)
		time.Sleep(1 * time.Second)
	}
}

func promptSwitchProfile(profMgr *profile.Manager) {
	termWidth, _ := getTerminalSize()
	if termWidth < 80 {
		termWidth = 80
	}
	fmt.Print(ClearScreen)
	title := "Switch Active Profile Node"
	pad := (termWidth - 2 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Println(BorderColor + "╭" + strings.Repeat("─", termWidth-2) + "╮" + Reset)
	fmt.Println(BorderColor + "│" + Reset + strings.Repeat(" ", pad) + AccentAmber + Bold + title + Reset + strings.Repeat(" ", termWidth-2-len(title)-pad) + BorderColor + "│" + Reset)
	fmt.Println(BorderColor + "╰" + strings.Repeat("─", termWidth-2) + "╯" + Reset)
	profiles := profMgr.List()
	for i, p := range profiles {
		tag := ""
		if p.Active {
			tag = AccentGreen + Bold + " [ACTIVE]" + Reset
		}
		fmt.Printf("  %s%d.%s %-24s %-24s %s\n", AccentAmber, i+1, Reset, p.Name, TextMuted+p.Server+Reset, tag)
	}
	fmt.Print(TextMuted + "\nEnter number to activate (or 'c' to cancel) ❯ " + Reset)
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
		fmt.Printf("\n"+AccentRose+"[-] Error: %v"+Reset+"\n", err)
		time.Sleep(2 * time.Second)
		return
	}

	fmt.Printf("\n"+AccentGreen+"[+] Switched active profile to [%s]!"+Reset+"\n", selected.Name)

	if daemon.IsDaemonRunning() {
		fmt.Println(AccentAmber + "[*] Reconnecting daemon with new profile..." + Reset)
		_ = daemon.StopBackgroundDaemon()
		time.Sleep(500 * time.Millisecond)
		_ = daemon.StartBackgroundDaemon()
	}
	time.Sleep(1 * time.Second)
}

func showLogsScreen() {
	termWidth, _ := getTerminalSize()
	if termWidth < 80 {
		termWidth = 80
	}
	resp, err := daemon.SendIPCCommand(daemon.IPCRequest{Command: "LOGS"}, 2*time.Second)
	fmt.Print(ClearScreen)
	title := "Live Connection Event Stream (Last 50 Events)"
	pad := (termWidth - 2 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Println(BorderColor + "╭" + strings.Repeat("─", termWidth-2) + "╮" + Reset)
	fmt.Println(BorderColor + "│" + Reset + strings.Repeat(" ", pad) + AccentCyan + Bold + title + Reset + strings.Repeat(" ", termWidth-2-len(title)-pad) + BorderColor + "│" + Reset)
	fmt.Println(BorderColor + "╰" + strings.Repeat("─", termWidth-2) + "╯" + Reset)
	if err != nil || resp == nil || len(resp.Logs) == 0 {
		fmt.Println("\n" + AccentAmber + "[No logs available or VPN not running]" + Reset)
	} else {
		for _, l := range resp.Logs {
			fmt.Println("  " + colorizeLog(l))
		}
	}
	fmt.Println("\n" + TextMuted + "Press Enter to return to Dashboard..." + Reset)
}

// Visual Helpers

func wrapBoxLine(content string, boxWidth int) string {
	innerWidth := boxWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}
	vis := len([]rune(stripANSI(content)))
	if vis > innerWidth {
		content = truncateANSI(content, innerWidth)
		vis = len([]rune(stripANSI(content)))
	}
	pad := innerWidth - vis
	if pad < 0 {
		pad = 0
	}
	return BorderColor + "│ " + Reset + content + strings.Repeat(" ", pad) + BorderColor + " │" + Reset
}

func truncateANSI(str string, maxVis int) string {
	var sb strings.Builder
	inEsc := false
	vis := 0
	for _, r := range str {
		if r == '\033' {
			inEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if vis >= maxVis {
			break
		}
		sb.WriteRune(r)
		vis++
	}
	sb.WriteString(Reset)
	return sb.String()
}

func padRight(s string, width int) string {
	vis := len([]rune(stripANSI(s)))
	if vis >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vis)
}

func stripANSI(str string) string {
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

func renderClockRows(t time.Time) [3]string {
	timeStr := t.Format("15:04")
	var r0, r1, r2 strings.Builder
	for _, r := range timeStr {
		glyph, ok := digitRows[r]
		if !ok {
			glyph = digitRows[' ']
		}
		r0.WriteString(glyph[0] + " ")
		r1.WriteString(glyph[1] + " ")
		r2.WriteString(glyph[2] + " ")
	}
	return [3]string{
		GradClock0 + r0.String() + Reset,
		GradClock1 + r1.String() + Reset,
		GradClock2 + r2.String() + Reset,
	}
}

func renderBar(current float64, max float64, barWidth int) string {
	if max <= 0 {
		max = 1
	}
	ratio := current / max
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled
	return AccentGreen + strings.Repeat("█", filled) + TextMuted + strings.Repeat("░", empty) + Reset
}

func renderMultiLineChart(history []float64, width int, height int) []string {
	chartRows := make([]string, height)
	peak := 1024.0 // 1KB min baseline
	for _, v := range history {
		if v > peak {
			peak = v
		}
	}

	labels := []string{
		fmt.Sprintf("%5s ┼ ", "0 B"),
		fmt.Sprintf("%5s ┤ ", formatBytes(peak*0.33)),
		fmt.Sprintf("%5s ┤ ", formatBytes(peak*0.66)),
		fmt.Sprintf("%5s ┤ ", formatBytes(peak)),
	}

	data := make([]float64, width)
	if len(history) > width {
		copy(data, history[len(history)-width:])
	} else {
		copy(data[width-len(history):], history)
	}

	blocks := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	for r := height - 1; r >= 0; r-- {
		var sb strings.Builder
		sb.WriteString(TextMuted + labels[r] + Reset)

		rowBottom := float64(r) / float64(height)
		rowTop := float64(r+1) / float64(height)

		for _, val := range data {
			ratio := val / peak
			if ratio <= rowBottom {
				sb.WriteString(TextMuted + " " + Reset)
			} else if ratio >= rowTop {
				sb.WriteString(AccentCyan + "█" + Reset)
			} else {
				frac := (ratio - rowBottom) / (rowTop - rowBottom)
				idx := int(math.Floor(frac * float64(len(blocks)-1)))
				if idx < 0 {
					idx = 0
				}
				if idx >= len(blocks) {
					idx = len(blocks) - 1
				}
				sb.WriteString(AccentCyan + string(blocks[idx]) + Reset)
			}
		}
		chartRows[r] = sb.String()
	}

	res := make([]string, height+1)
	for i := 0; i < height; i++ {
		res[i] = chartRows[height-1-i]
	}
	res[height] = TextMuted + "      └" + BorderColor + strings.Repeat("─", width) + Reset

	return res
}

func colorizeLog(l string) string {
	l = stripANSI(l)
	l = strings.TrimSpace(l)
	if strings.HasPrefix(l, "[") {
		if endIdx := strings.Index(l, "] "); endIdx != -1 {
			timePart := l[:endIdx+1]
			msgPart := strings.TrimSpace(l[endIdx+2:])
			msgPart = cleanSingboxEvent(msgPart)
			return TextMuted + timePart + " " + colorizeMsg(msgPart)
		}
	}
	return colorizeMsg(cleanSingboxEvent(l))
}

func cleanSingboxEvent(m string) string {
	if strings.HasPrefix(m, "[") {
		if closeIdx := strings.Index(m, "] "); closeIdx != -1 {
			bracketContent := m[1:closeIdx]
			rest := strings.TrimSpace(m[closeIdx+2:])
			fields := strings.Fields(bracketContent)
			if len(fields) >= 2 {
				return "(" + fields[1] + ") " + rest
			}
			return rest
		}
	}
	return m
}

func colorizeMsg(m string) string {
	lower := strings.ToLower(m)
	if strings.Contains(lower, "dns") || strings.Contains(lower, "exchanged") {
		return AccentCyan + m + Reset
	}
	if strings.Contains(lower, "outbound") || strings.Contains(lower, "vless") || strings.Contains(lower, "tcp") || strings.Contains(lower, "connected") {
		return AccentGreen + m + Reset
	}
	if strings.Contains(lower, "block") || strings.Contains(lower, "error") || strings.Contains(lower, "closed") {
		return AccentRose + m + Reset
	}
	return TextMain + m + Reset
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
	return fmt.Sprintf("%.1f%c", bytes/div, "KMGTPE"[exp])
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
	if maxLen < 4 {
		maxLen = 4
	}
	if len([]rune(s)) > maxLen {
		return string([]rune(s)[:maxLen-3]) + "..."
	}
	return s
}
