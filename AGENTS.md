# 🤖 NuxMod Developer & AI Agent Guide (`AGENTS.md`)

> **Project Name:** NuxMod (formerly vless-linux)  
> **Author & Maintainer:** Sathira Laksahan  
> **Repository:** `https://github.com/Sathira-Laksahan/vless-linux`  
> **Target OS:** Linux (Debian, Ubuntu, Kali, Arch, Fedora, openSUSE)  
> **Language:** Go (1.22+, Zero External Go Dependencies)  
> **Core Engine:** Sing-box (`v1.8.14+`, gVisor TUN stack)  

---

## 1. Project Overview & Philosophy

**NuxMod** is a high-performance, native Linux system-wide VPN & DPI bypass client with an interactive **Tokyo Night / Bluloco Terminal User Interface (TUI)**. It serves as the Linux equivalent of Windows tools like **NetMod Syna**, **Clash Verge**, or **NekoRay**.

### Core Tenets:
1. **100% System-Wide TUN Routing:** All L3 network traffic (Browsers, Terminal, Docker, cURL, Steam) is routed through a virtual `tun0` adapter with Google gVisor user-space network stack.
2. **Terminal-Close Protection:** The VPN daemon runs in a detached background process (`Setsid: true`). Closing the terminal window **never** drops the VPN connection.
3. **Zero External Go Dependencies:** The Go codebase uses **only the Go Standard Library**. It compiles instantly and statically with `CGO_ENABLED=0`.
4. **Pure ANSI Responsive TUI:** The TUI dashboard uses 24-bit TrueColor ANSI, alternate screen buffers (`\033[?1049h`), and atomic buffer writes to guarantee **zero terminal scrolling and zero flickering**.

---

## 2. Codebase Architecture & Directory Map

```text
.
├── cmd/
│   ├── nuxmod/             # Primary CLI / TUI entrypoint (nuxmod binary)
│   │   └── main.go
│   └── vless-linux/        # Backwards-compatible legacy entrypoint
│       └── main.go
├── internal/
│   ├── config/             # Sing-box JSON generator, DNS fallback, outbound routing
│   │   ├── generator.go
│   │   └── types.go
│   ├── daemon/             # Background daemon process, Unix IPC socket, /sys bandwidth monitor
│   │   ├── daemon.go
│   │   └── ipc.go
│   ├── engine/             # Sing-box binary locator, auto-downloader, lifecycle manager
│   │   └── manager.go
│   ├── parser/             # vless:// URI parser (UUID, TLS, SNI spoofing, uTLS Chrome fingerprint)
│   │   └── parser.go
│   ├── profile/            # Profile storage (/etc/nuxmod/profiles.json) with auto-migration
│   │   └── manager.go
│   ├── routing/            # Linux kernel routes, IPv6 mitigation, TLS handshake ping probe
│   │   └── routing.go
│   ├── tui/                # Tokyo Night / Bluloco 4-panel dashboard with live sparkline wave
│   │   └── dashboard.go
│   ├── ui/                 # CLI banners, formatted success/error messages
│   │   └── terminal.go
│   └── updater/            # GitHub Releases self-updater & in-place binary installer
│       └── updater.go
├── scripts/
│   └── install.sh          # Global system installer & 1-line curl script
├── dist/                   # Release packages (.tar.gz archives)
├── LICENSE                 # MIT License (Copyright Sathira Laksahan)
├── README.md               # User-facing documentation
└── AGENTS.md               # This technical reference guide
```

---

## 3. Key Components & How They Work

### 3.1. Background Daemon & IPC Socket (`internal/daemon`)
- When a user connects (`nuxmod connect` or pressing `c` in the TUI), the main process executes `nuxmod __daemon` as a detached subprocess with `SysProcAttr: &syscall.SysProcAttr{Setsid: true}`.
- The daemon creates a Unix Domain Socket at `/var/run/nuxmod.sock` (fallback: `/tmp/nuxmod-<uid>.sock`).
- Any CLI command (`nuxmod status`, `nuxmod logs`) communicates with this daemon via JSON-encoded IPC messages (`IPCRequest` / `IPCResponse`).
- Live bandwidth statistics are read directly from the Linux kernel at `/sys/class/net/tun0/statistics/rx_bytes` and `tx_bytes`.

### 3.2. Sing-box & gVisor TUN Integration (`internal/config`)
- Configuration is generated dynamically as a Sing-box JSON document:
  - **Inbound:** `type: "tun"`, `interface_name: "tun0"`, `inet4_address: "172.19.0.1/30"`, `stack: "gvisor"`, `mtu: 9000`, `auto_route: true`, `strict_route: false`.
  - **Outbound:** `type: "vless"`, `security: "tls"`, `tls.sni: "<spoofed_domain>"`, `tls.utls.fingerprint: "chrome"`, `tls.insecure: true`.
  - **DNS:** Encrypted remote resolver `1.1.1.1` (TLS/DoH), routed through the VLESS tunnel to prevent ISP DNS poisoning.
  - **Direct Bypasses:** Private IP ranges (`192.168.0.0/16`, `10.0.0.0/8`, `172.16.0.0/12`, `127.0.0.0/8`) are routed `direct` so local LAN / SSH connections never drop.

### 3.3. TUI Dashboard Engine (`internal/tui/dashboard.go`)
- **Alternate Screen Buffer:** On launch, prints `\033[?1049h\033[H\033[2J`. This isolates the dashboard from the user's terminal scrollback history. On exit (`q` or `x`), prints `\033[?1049l\033[?25h` to cleanly restore the terminal.
- **Dynamic Responsive Width:** Calls `getTerminalSize()` via `TIOCGWINSZ` ioctl on `/dev/tty` or `os.Stdout`. Splits terminal columns into two panels: `wLeft = (termWidth - 2) / 2` and `wRight = termWidth - 2 - wLeft`.
- **In-Place Atomic Rendering:** Redraws are built into a `strings.Builder`, starting with `\033[H` (Cursor Home), printing lines with `\033[K` (Clear to EOL), and clearing trailing space with `\033[J`. This prevents flickering and stops terminal scrolling.
- **ANSI Width Safety:** Fixed-width box borders (`wrapBoxLine`) use `stripANSI()` and `truncateANSI()` to prevent escape sequences from throwing off character counts.

---

## 4. Critical Rules & Known Gotchas

> [!CAUTION]
> **1. IPv6 Deadlock & Multi-VPN Coexistence:**  
> On Linux bridged networking (VMware, VirtualBox, or native LAN), upstream ISPs often lack native IPv6 or drop external IPv6 traffic. However, disabling IPv6 globally via `all/disable_ipv6 = 1` crashes secondary VPN clients (such as OpenVPN for Hack The Box / TryHackMe or WireGuard) when they receive pushed IPv6 configs.  
> **Always mitigate IPv6 via blackhole routing** (`ip -6 route replace unreachable default metric 1`) while keeping kernel IPv6 enabled (`disable_ipv6 = 0`). This guarantees instant IPv4 fallback for public web traffic without breaking secondary TUN/TAP interfaces.

> [!IMPORTANT]
> **2. Context Propagation:**  
> Never pass `nil` context into `daemon.Run()` or `engineMgr.Start()`. Always initialize with `context.Background()`.

> [!WARNING]
> **3. Stdin Collision in TUI:**  
> Do not launch multiple goroutines reading `os.Stdin`. Use the single `inputChan` state machine in `RunDashboard()` to avoid swallowed keystrokes.

> [!TIP]
> **4. Binary Replacement on Linux:**  
> When replacing the running `/usr/local/bin/nuxmod` binary, `cp` will fail with `Text file busy`. Always use `install -m 755 <source> /usr/local/bin/nuxmod` because `install` unlinks the inode before writing.

---

## 5. Build, Test & Packaging Commands

### Cross-Compile for Linux:
```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -ldflags="-s -w" -o nuxmod ./cmd/nuxmod
```

### Build Standalone Release Archive:
```bash
mkdir -p dist
tar -czvf dist/nuxmod-v2.0.0-linux-amd64.tar.gz nuxmod scripts/install.sh README.md LICENSE
```

### Test on Linux Target (via SSH):
```bash
sudo ./scripts/install.sh
sudo nuxmod status
sudo nuxmod
```
