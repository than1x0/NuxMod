# 🚀 VLESS Linux — System-Wide TUN Bypass & TUI Dashboard Tool (Go)

A high-performance, native Linux system-wide VPN & proxy bypass tool built with **Go (Golang)**. It converts any `vless://` link (with TLS, SNI spoofing, and Chrome uTLS fingerprints) into a full-OS virtual network adapter (`tun0`) on Linux — routing **Web Browsers, Terminal/Bash/Curl, Docker, and all background services**, with a rich **Terminal User Interface (TUI) Dashboard**, functioning like **NetMod Syna** on Linux!

---

## 🎯 Features

- **🌐 100% System-Wide TUN Routing:** Uses Linux Kernel `/dev/net/tun` with Google gVisor stack to route all L3 IP packets.
- **🖥️ Interactive Live TUI Dashboard:** Real-time speed gauges, ping monitor, node selector, and event logs inside terminal.
- **🔄 Detached Background Daemon:** Closing the terminal does **NOT** drop the VPN. Reopen terminal anytime to reattach!
- **📋 Multi-Profile Manager:** Save multiple VLESS nodes, switch between them, and delete unwanted profiles.
- **🤫 Zero Log Flooding:** Verbose engine logs are captured in a memory ring buffer and displayed cleanly in the mini-log box.
- **📊 Real-Time Bandwidth Monitor:** Measures live download/upload speeds (MB/s & KB/s) directly from `/sys/class/net/tun0`.
- **🛡️ DPI Bypass & SNI Spoofing:** Seamlessly handles SNI spoofing (`aka.ms`, etc.) and TLS fingerprints (`fp=chrome`).
- **📦 Self-Contained System Tool:** Installs globally into `/usr/local/bin/` and `/etc/vless-linux/` — setup folder can be deleted!

---

## 🚀 Installation & Quick Start

### 1-Click System Installation (Recommended)

1. Clone or download the repository on your Linux machine:
   ```bash
   git clone https://github.com/Sathira-Laksahan/vless-linux.git
   cd vless-linux
   ```
2. Run the installer as root:
   ```bash
   sudo ./scripts/install.sh
   ```
3. **Setup folder එක delete කළ හැක:** Installation එක අවසන් වූ පසු, source folder එක සම්පූර්ණයෙන්ම `rm -rf` කර delete කළ හැක. Tool එක Linux system එකේ `/usr/local/bin/` හි ස්ථිරවම ස්ථාපිත වේ.

---

## 💻 Usage & Commands

### 1. Interactive TUI Dashboard (Default)
Terminal එකේ සරලවම පහත විධානය run කරන්න:
```bash
sudo vless-linux
```
* Status indicator (Connected / Disconnected)
* Real-time Download / Upload speeds
* Node name, Server IP, Latency (ping)
* Saved profiles list
* Recent system logs
* **Hotkeys:**
  * `[c]` -> Connect / Disconnect VPN
  * `[p]` -> Switch active profile
  * `[a]` -> Add new VLESS profile
  * `[d]` -> Delete a profile
  * `[q]` -> Close dashboard (**VPN එක background එකේ දිගටම run වේ!**)
  * `[x]` -> Disconnect and Exit

---

### 2. Background Connection (Terminal Close Protection)

```bash
# Saved active profile එකට background එකේ connect වීම
sudo vless-linux connect

# අලුත් VLESS link එකක් ලබා දී connect වීම (Single quotes භාවිතා කරන්න)
sudo vless-linux connect 'vless://uuid@server:443?encryption=none&fp=chrome&security=tls&sni=aka.ms&type=tcp#NodeName'

# Connection එක නවතා routes restore කිරීම
sudo vless-linux disconnect
```
> **💡 සටහන:** `sudo vless-linux connect` කළ පසු Terminal එක close කළද connection එක කිසිසේත් විසන්ධි නොවේ!

---

### 3. Profile Management Commands

```bash
# Profiles ලැයිස්තුව බැලීම
vless-linux profile list

# අලුත් VLESS profile එකක් add කිරීම
vless-linux profile add '<vless_url>' "Dialog-Bypass"

# Active profile එක මාරු කිරීම (Number හෝ Name මගින්)
vless-linux profile use 1

# Profile එකක් delete කිරීම
vless-linux profile delete 2
```

---

### 4. Status & Logs

```bash
# තත්ත්වය සහ Bandwidth පරීක්ෂා කිරීම (root අවශ්‍ය නොවේ)
vless-linux status

# පසුගිය logs බැලීම
vless-linux logs
```

---

### 5. Auto-Connect on Linux Boot (systemd)

පරිගණකය On වන විටම Background එකේ auto-connect වීමට:
```bash
sudo systemctl enable --now vless-linux
```
Service status බැලීමට:
```bash
sudo systemctl status vless-linux
```
