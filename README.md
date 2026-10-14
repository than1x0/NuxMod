# 🚀 VLESS Linux — System-Wide TUN Bypass Tool (Go)

A high-performance, native Linux system-wide VPN & proxy bypass tool built with **Go (Golang)**. It converts any `vless://` link (with TLS, SNI spoofing, and Chrome uTLS fingerprints) into a full-OS virtual network adapter (`tun0`) on Linux — routing **Web Browsers, Terminal/Bash/Curl, Docker, and all background services**, exactly like **NetMod Syna** on Windows!

---

## 🎯 Features

- **🌐 100% System-Wide TUN Routing:** Uses Linux Kernel `/dev/net/tun` with Google gVisor TCP/IP stack to route all L3 IP packets.
- **🛡️ DPI Bypass & SNI Spoofing:** Seamlessly handles SNI spoofing (`aka.ms`, `zoom.us`, etc.) and TLS Client Hello fingerprints (`fp=chrome`).
- **🔒 DNS Poisoning Protection:** Routes DNS over TLS (`tls://1.1.1.1`) directly through the VLESS tunnel, bypassing ISP DNS hijack (Port 53).
- **⚡ Anti-Loop Protection:** Automatically detects the default network gateway and adds bypass routes for the proxy server host, preventing connection freezes.
- **📦 Single Standalone Binary:** Zero external runtime dependencies (no Python, no Node.js required).
- **🔄 Auto Core Provisioning:** Automatically downloads and configures the verified Sing-box 1.8+ Linux engine if not found.
- **⚙️ systemd Service Ready:** Run as a background daemon that connects automatically when Linux boots up.

---

## 📂 Project Structure

```
Vless/
├── cmd/
│   └── vless-linux/
│       └── main.go              # Main CLI entrypoint
├── internal/
│   ├── parser/
│   │   └── vless.go             # VLESS URI decoder & validator
│   ├── config/
│   │   └── generator.go         # TUN + Sing-box JSON generator
│   ├── engine/
│   │   └── manager.go           # Process lifecycle & signal handler
│   ├── routing/
│   │   └── linux.go             # Linux TUN & latency checks
│   └── ui/
│       └── terminal.go          # ANSI color terminal UI
├── scripts/
│   ├── install.sh               # 1-Click Linux installer
│   └── vless-linux.service      # systemd unit file
├── vless-linux                  # Precompiled Linux (amd64) binary
├── vless-windows.exe            # Windows testing binary
├── go.mod
└── README.md
```

---

## 🚀 Quick Start (Linux මත පාවිච්චි කරන ආකාරය)

### ක්‍රමය 1: සකස් කර ඇති Linux Binary එක කෙලින්ම Run කිරීම (Fastest)

1. මෙම ෆෝල්ඩරයේ ඇති **`vless-linux`** binary එක ඔබේ Linux පරිගණකයට Copy කරගන්න.
2. Terminal එක විවෘත කර Binary එකට execute permissions ලබා දෙන්න:
   ```bash
   chmod +x vless-linux
   ```
3. පහත විධානය මගින් ඔබගේ VLESS link එක සමඟ Connect වන්න:
   ```bash
   sudo ./vless-linux connect "vless://<your-uuid>@<your-server>:443?encryption=none&fp=chrome&security=tls&sni=<your-sni>&type=tcp#Node-Name"
   ```
4. **සම්බන්ධතාවය අවසන් කිරීමට (Disconnect):** Terminal එකේ `Ctrl + C` ඔබන්න. Tool එක මගින් ස්වයංක්‍රීයව Routing Tables සහ DNS නැවත සාමාන්‍ය තත්වයට පත් කරයි.

> **💡 Note:** එක් වරක් Link එක ලබා දුන් පසු එය සුරැකෙන බැවින්, ඊළඟ වතාවේදී සරලව `sudo ./vless-linux connect` run කළ හැක!

---

### ක්‍රමය 2: සම්පූර්ණ System Service එකක් ලෙස Install කිරීම (1-Click Installer)

`scripts/install.sh` මගින් `/usr/local/bin/vless-linux` ලෙස install කර systemd service එකක් සාදාගත හැක:

```bash
chmod +x scripts/install.sh
sudo ./scripts/install.sh
```

**Auto-connect on Linux boot (පරිගණකය On වන විටම auto connect වීමට):**
```bash
sudo systemctl enable --now vless-linux
```

**Service Status පරීක්ෂා කිරීමට:**
```bash
sudo systemctl status vless-linux
```

---

## 🛠️ CLI Commands & Usage

| Command | විස්තරය |
| :--- | :--- |
| `sudo ./vless-linux connect [link]` | මුළු OS එකම TUN mode හරහා VLESS වෙත සම්බන්ධ කරයි |
| `./vless-linux parse <link>` | VLESS link එකේ තොරතුරු (UUID, Host, SNI, fp) පරීක්ෂා කරයි |
| `./vless-linux test <link>` | Server එකේ latency සහ connection එක test කරයි |
| `./vless-linux export <link> [file.json]` | Sing-box JSON configuration එක export කරගැනීමට |
| `./vless-linux version` | Tool එකේ version එක පෙන්වයි |

---

## 🔨 Building from Source (කේතය නැවත Compile කිරීමට)

### Linux මත Build කිරීම:
```bash
go build -ldflags="-s -w" -o vless-linux ./cmd/vless-linux
```

### Windows මත සිට Linux සඳහා Cross-compile කිරීම:
```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o vless-linux ./cmd/vless-linux
```
