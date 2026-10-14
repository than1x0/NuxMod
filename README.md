# ⚡ NuxMod — Linux System-Wide TUN Bypass & VPN Client

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Linux%20(Debian%20%7C%20Ubuntu%20%7C%20Kali%20%7C%20Arch%20%7C%20Fedora)-blue?style=for-the-badge&logo=linux" alt="Platform Linux" />
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go" alt="Go 1.22+" />
  <img src="https://img.shields.io/badge/License-MIT-emerald?style=for-the-badge" alt="MIT License" />
  <img src="https://img.shields.io/badge/Version-2.0.0-rose?style=for-the-badge" alt="Version 2.0.0" />
  <img src="https://img.shields.io/badge/Author-Sathira%20Laksahan-amber?style=for-the-badge" alt="Author Sathira Laksahan" />
</p>

---

## 🎯 About NuxMod

**NuxMod** is a high-performance, native Linux system-wide VPN & DPI bypass client built with **pure Go**. It functions as the ultimate Linux alternative to Windows tools like **NetMod Syna**, **Clash**, or **NekoRay**.

With **NuxMod**, you can import any standard `vless://` link (complete with TLS SNI spoofing, WebSocket/gRPC transport, and Chrome uTLS fingerprints) and route your **entire operating system** through a virtual L3 network adapter (`tun0`) powered by Google's **gVisor user-space network stack**.

Browsers, Terminal sessions, cURL, Docker containers, Steam, and background system services are all seamlessly and transparently proxied!

---

## ✨ Features at a Glance

* **🌐 100% Full-OS L3 TUN Routing:** Employs `/dev/net/tun` with gVisor to capture and encrypt all system-wide IP traffic.
* **🛡️ Terminal-Close Protection:** The VPN daemon runs in a detached background process (`Setsid: true`). Closing your terminal **never** drops the VPN connection!
* **🖥️ Responsive Tokyo Night / Bluloco TUI Dashboard:**
  * 24-bit TrueColor multi-panel terminal interface matching modern developer aesthetics.
  * Real-time ASCII/Unicode traffic wave chart (multi-line sparkline graph).
  * Centered 3-row digital block clock with vertical color gradient.
  * Live bandwidth rate gauges (MB/s and KB/s) read directly from kernel statistics.
  * Alternate Screen Buffer (`\033[?1049h`) ensuring **zero terminal scrolling** and **zero flickering**.
* **📋 Multi-Profile Manager:** Save multiple VLESS configurations (`/etc/nuxmod/profiles.json`), switch active nodes on the fly, and delete obsolete nodes.
* **🔒 Encrypted DNS & Leak Prevention:** Built-in encrypted DNS resolver (`1.1.1.1` via TLS) with automatic IPv6 mitigation prevents ISP DNS hijacking and network deadlocks.
* **⚡ Standalone & Zero Dependencies:** Compiled with pure standard library Go — requires no runtime dependencies other than the included Sing-box core.
* **🚀 1-Line Universal Installer:** Install globally on any modern Linux distribution with a single command.

---

## 🖥️ Live TUI Dashboard Preview

```text
╭─ [C]lock ─ Connection ───────────────────────────╮  ╭─ [P]rofiles ─ Routing ──────────────────[nuxmod]─╮
│                ██ ███      ██ ███                │  │  #  NAME             SERVER            PING      │
│                ██ ▄██  ▄   ██ ██▄                │  │  ● 1. Bypass-Node     lw03.sahanwic...  5ms    A │
│                ██ ███  ▄   ██ ███                │  │    Endpoint  : lw03.sahanwickramasinghe.me:443   │
│           14:20:15 · Wed Oct 14, 2026            │  │    TLS SNI   : aka.ms (Security TLS Handshake)   │
│ ─────────────── Network Status ───────────────   │  │ ──────────────────────────────────────────────── │
│ State   : ● CONNECTED (tun0 / gVisor)            │  │ Routing : Full OS TUN Bypass (tun0 -> 172.19.0.1 │
│ Node    : Bypass-Node (aka.ms)                   │  │ DNS     : 1.1.1.1 (TLS Encrypted Remote Upstream │
│ Ping    : 5ms  Uptime: 01h 14m 22s               │  │ Bypass  : LAN Direct (192.168.0.0/16, 10.0.0.0/8 │
│ [c] toggle  [p] switch  [r] ping                 │  │ [a] add link  [d] delete  [p] select             │
╰──────────────────────────────────────────────────╯  ╰──────────────────────────────────────────────────╯

╭─ [R]esources ─ TUN ───────────up 01h 14m 22s · tun0─╮  ╭─ [T]raffic Wave ─ Events ──────────────────Live Wave─╮
│ Bandwidth Throughput & Realtime Rates:           │  │  2.4M ┤                      ▂▆       ▃█▅         │
│ DL : 1.4 MB/s   ██████████░░ Pk: 3.8 MB/s        │  │  1.6M ┤                 ▆▅   ██▆     ▃███         │
│ UL : 180.2 KB/s ████░░░░░░░░ Pk: 450.0 KB/s      │  │ 800 K ┤                ▆██▅▂████▂   ▃████▅       │
│ ──────────────────────────────────────────────── │  │   0 B ┼            ▂▃▅███████████▄▃▅██████▆       │
│ Data In  : ⬇ 482.5 MB    Peak In  : ⬇ 3.8 MB/s   │  │       └────────────────────────────────────────  │
│ Data Out : ⬆ 34.2 MB     Peak Out : ⬆ 450.0 KB/s │  │ ──────────────────────────────────────────────── │
│ Adapter  : tun0 (172.19.0.1/30, MTU 9000, gVisor │  │ ● [14:20:12] (182ms) outbound/vless [tls connected] │
│ Core     : sing-box 1.8.14 (amd64-linux, active  │  │ ● [14:20:10] (0ms) dns: exchanged aka.ms         │
│ [c] connect  [l] logs  [r] refresh               │  │ [l] full logs  [q] detach  [x] quit              │
╰──────────────────────────────────────────────────╯  ╰──────────────────────────────────────────────────╯

nuxmod v2.0 │ 14:20:15 │ Focus: tun0:connected │ Scheme: bluloco │ c:toggle p:switch a:add d:del l:logs q:detach x:exit

❯ 
```

---

## 🚀 Quick Start & Installation

### Option 1: 1-Line Universal Command (Recommended)

Run the following command in any Linux terminal (Debian, Ubuntu, Kali, Arch, Fedora, etc.):

```bash
curl -fsSL https://raw.githubusercontent.com/Sathira-Laksahan/vless-linux/main/scripts/install.sh | sudo bash
```

This script automatically:
1. Detects your CPU architecture (`amd64` or `arm64`).
2. Configures `/dev/net/tun` kernel permissions.
3. Installs the Sing-box core engine (`v1.8.14`).
4. Installs the `nuxmod` binary globally to `/usr/local/bin/nuxmod`.
5. Creates a backward-compatible `/usr/local/bin/vless-linux` symlink.
6. Sets up `/etc/systemd/system/nuxmod.service`.

---

### Option 2: Standalone Release Archive (`.tar.gz`)

For offline installation or manual distribution:

1. Download the release archive (`nuxmod-v2.0.0-linux-amd64.tar.gz`).
2. Extract and run the installer:
   ```bash
   tar -xzf nuxmod-v2.0.0-linux-amd64.tar.gz
   cd nuxmod
   sudo ./scripts/install.sh
   ```
3. *Note:* Once installed, you can safely delete the source/download folder!

---

## 💻 Commands & Usage

### 1. Launch the Interactive TUI Dashboard (Default)

```bash
sudo nuxmod
```

#### Dashboard Hotkeys:
| Key | Action | Description |
|:---:|:---|:---|
| `c` | **Toggle Connect** | Connects or disconnects the background VPN daemon |
| `p` | **Switch Profile** | Opens interactive profile switcher menu |
| `a` | **Add Node** | Prompts to paste a new `vless://` link and save it |
| `d` | **Delete Node** | Removes an unwanted profile from `/etc/nuxmod/profiles.json` |
| `l` | **Live Logs** | Views the last 50 detailed connection & DNS query events |
| `u` | **Update Tool** | Checks GitHub and performs an in-place self-upgrade |
| `r` | **Refresh** | Forces an immediate screen and bandwidth poll |
| `q` | **Detach** | **Exits dashboard while VPN continues running in the background!** |
| `x` | **Exit & Kill** | Disconnects VPN, restores original network routes, and exits |

---

### 2. Command-Line (CLI) Shortcuts

```bash
# Check GitHub and self-update NuxMod to the latest version
sudo nuxmod update

# Connect to your active saved profile in the background
sudo nuxmod connect

# Connect using a new VLESS link directly (Wrap link in single quotes!)
sudo nuxmod connect 'vless://uuid@server:443?encryption=none&fp=chrome&security=tls&sni=aka.ms&type=tcp#NodeName'

# Disconnect VPN and restore original ISP routing
sudo nuxmod disconnect

# Check live connection status and speed (No root required!)
nuxmod status

# View recent network logs
nuxmod logs
```

---

### 3. Profile Management via CLI

```bash
# List all saved profiles
nuxmod profile list

# Add a new profile
nuxmod profile add '<vless_url>' "My-Fast-Node"

# Switch active profile by number or name
nuxmod profile use 1

# Delete a profile by number or ID
nuxmod profile delete 2
```

---

### 4. Background Service (Systemd)

NuxMod installs a ready-to-use systemd service:

```bash
# Start NuxMod as a background service
sudo systemctl start nuxmod

# Enable NuxMod to start automatically on system boot
sudo systemctl enable nuxmod

# Check service status
sudo systemctl status nuxmod
```

---

## 🔧 Troubleshooting

### 1. "Text file busy" during updates
If updating the binary while the background daemon is active, always use:
```bash
sudo install -m 755 nuxmod /usr/local/bin/nuxmod
```

### 2. Local LAN / SSH disconnects
NuxMod includes built-in bypass rules for private subnets (`192.168.0.0/16`, `10.0.0.0/8`, `172.16.0.0/12`). Local SSH sessions, Samba shares, and printer networks remain direct and unaffected.

---

## 👤 Author & Credits

- **Developer:** [Sathira Laksahan](https://github.com/Sathira-Laksahan)
- **Engine:** [Sing-box](https://github.com/SagerNet/sing-box) & Google gVisor
- **License:** [MIT License](LICENSE) — Copyright (c) 2026 Sathira Laksahan
