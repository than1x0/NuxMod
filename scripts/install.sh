#!/usr/bin/env bash
# ==============================================================================
# NuxMod — Linux System-Wide TUN Bypass & VPN Client
# Developed by: Sathira Laksahan
# GitHub: https://github.com/Sathira-Laksahan/vless-linux
# ==============================================================================
set -e

# Terminal colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}"
echo "  _   _             __  __           _ "
echo " | \ | |           |  \/  |         | |"
echo " |  \| |_   ___  __| \  / | ___   __| |"
echo " | . \` | | | \ \/ /| |\/| |/ _ \ / _\` |"
echo " | |\  | |_| |>  < | |  | | (_) | (_| |"
echo " |_| \_|\__,_/_/\_\|_|  |_|\___/ \__,_|"
echo -e "${NC}"
echo -e "${BLUE}======================================================${NC}"
echo -e "${GREEN}${BOLD}   NuxMod — System-Wide TUN Bypass & VPN Client       ${NC}"
echo -e "${CYAN}   Developed by Sathira Laksahan                     ${NC}"
echo -e "${BLUE}======================================================${NC}\n"

# 1. Root privilege verification
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}[-] Please run this installer as root:${NC} sudo $0"
  exit 1
fi

# 2. Architecture detection
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)
    GOARCH="amd64"
    ;;
  aarch64|arm64)
    GOARCH="arm64"
    ;;
  *)
    echo -e "${RED}[-] Unsupported CPU architecture: $ARCH${NC}"
    exit 1
    ;;
esac
echo -e "${YELLOW}[*] Detected architecture:${NC} linux-${GOARCH}"

# 3. Kernel TUN device verification
if [ ! -c /dev/net/tun ]; then
  echo -e "${YELLOW}[*] Creating /dev/net/tun device node...${NC}"
  mkdir -p /dev/net
  mknod /dev/net/tun c 10 200
  chmod 666 /dev/net/tun
fi

# 4. System config directories & migration
mkdir -p /etc/nuxmod
chmod 755 /etc/nuxmod

# Migrate from legacy /etc/vless-linux if present
if [ -f "/etc/vless-linux/profiles.json" ] && [ ! -f "/etc/nuxmod/profiles.json" ]; then
  echo -e "${YELLOW}[*] Migrating existing profiles from /etc/vless-linux...${NC}"
  cp /etc/vless-linux/profiles.json /etc/nuxmod/profiles.json
fi

# 5. Sing-box core binary installation
SINGBOX_VERSION="1.8.14"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." 2>/dev/null && pwd || echo "")"

if [ -f "/usr/local/bin/sing-box" ]; then
  echo -e "${GREEN}[+] Sing-box core already present at /usr/local/bin/sing-box${NC}"
elif [ -f "/root/.nuxmod/bin/sing-box" ]; then
  echo -e "${YELLOW}[*] Installing Sing-box core from cache...${NC}"
  cp /root/.nuxmod/bin/sing-box /usr/local/bin/sing-box
  chmod +x /usr/local/bin/sing-box
elif [ -f "/root/.vless-linux/bin/sing-box" ]; then
  echo -e "${YELLOW}[*] Installing Sing-box core from legacy cache...${NC}"
  cp /root/.vless-linux/bin/sing-box /usr/local/bin/sing-box
  chmod +x /usr/local/bin/sing-box
elif [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/sing-box" ]; then
  cp "$SCRIPT_DIR/sing-box" /usr/local/bin/sing-box
  chmod +x /usr/local/bin/sing-box
else
  echo -e "${YELLOW}[*] Downloading Sing-box core v${SINGBOX_VERSION} (${GOARCH})...${NC}"
  TMP_DIR=$(mktemp -d)
  SINGBOX_URL="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}.tar.gz"
  if curl -fsSL "$SINGBOX_URL" -o "$TMP_DIR/sing-box.tar.gz"; then
    tar -xzf "$TMP_DIR/sing-box.tar.gz" -C "$TMP_DIR"
    cp "$TMP_DIR"/sing-box-*/sing-box /usr/local/bin/sing-box
    chmod +x /usr/local/bin/sing-box
    echo -e "${GREEN}[+] Sing-box core installed successfully!${NC}"
  else
    echo -e "${RED}[-] Warning: Failed to download Sing-box directly.${NC}"
  fi
  rm -rf "$TMP_DIR"
fi

if [ -f "/usr/local/bin/sing-box" ]; then
  setcap cap_net_admin,cap_net_bind_service+ep /usr/local/bin/sing-box 2>/dev/null || true
fi

# 6. Install NuxMod binary to /usr/local/bin
INSTALLED=false

if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/nuxmod" ]; then
  echo -e "${YELLOW}[*] Installing nuxmod binary to /usr/local/bin...${NC}"
  install -m 755 "$SCRIPT_DIR/nuxmod" /usr/local/bin/nuxmod
  INSTALLED=true
elif [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/vless-linux" ]; then
  echo -e "${YELLOW}[*] Installing binary to /usr/local/bin/nuxmod...${NC}"
  install -m 755 "$SCRIPT_DIR/vless-linux" /usr/local/bin/nuxmod
  INSTALLED=true
elif command -v go &> /dev/null && [ -n "$SCRIPT_DIR" ] && [ -d "$SCRIPT_DIR/cmd" ]; then
  echo -e "${YELLOW}[*] Compiling NuxMod from source with Go...${NC}"
  cd "$SCRIPT_DIR"
  if [ -d "$SCRIPT_DIR/cmd/nuxmod" ]; then
    go build -ldflags="-s -w" -o /usr/local/bin/nuxmod ./cmd/nuxmod
  else
    go build -ldflags="-s -w" -o /usr/local/bin/nuxmod ./cmd/vless-linux
  fi
  chmod +x /usr/local/bin/nuxmod
  INSTALLED=true
else
  # Remote 1-line curl fallback: download pre-built binary
  echo -e "${YELLOW}[*] Downloading latest NuxMod binary...${NC}"
  if curl -fsSL "https://github.com/Sathira-Laksahan/vless-linux/releases/latest/download/nuxmod" -o /usr/local/bin/nuxmod 2>/dev/null; then
    chmod +x /usr/local/bin/nuxmod
    INSTALLED=true
  fi
fi

if [ "$INSTALLED" = false ]; then
  echo -e "${RED}[-] Error: Could not install nuxmod binary.${NC}"
  exit 1
fi

# Create backwards-compatible symlink for vless-linux
ln -sf /usr/local/bin/nuxmod /usr/local/bin/vless-linux

# Set capabilities so non-root socket operations work smoothly
setcap cap_net_admin,cap_net_bind_service+ep /usr/local/bin/nuxmod 2>/dev/null || true

# 7. Setup systemd service template
echo -e "${YELLOW}[*] Configuring systemd service (/etc/systemd/system/nuxmod.service)...${NC}"
cat << 'EOF' > /etc/systemd/system/nuxmod.service
[Unit]
Description=NuxMod System-Wide TUN Bypass Service
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nuxmod connect
Restart=always
RestartSec=3s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload 2>/dev/null || true

echo -e "\n${GREEN}======================================================${NC}"
echo -e "${GREEN}${BOLD}[+] NuxMod v2.0 Installed Successfully to /usr/local/bin/nuxmod!${NC}"
echo -e "${GREEN}======================================================${NC}"
echo -e "${CYAN}[i] You can now safely delete the setup folder if desired.${NC}\n"
echo -e "${BOLD}Quick Usage:${NC}"
echo -e "  ${BLUE}sudo nuxmod${NC}                       -> Launch Interactive TUI Dashboard"
echo -e "  ${BLUE}sudo nuxmod connect${NC}               -> Connect in background (terminal close protected)"
echo -e "  ${BLUE}sudo nuxmod disconnect${NC}            -> Disconnect & cleanly restore network"
echo -e "  ${BLUE}nuxmod status${NC}                     -> Check live speed and connection state"
echo -e "  ${BLUE}nuxmod profile add \"<url>\" [name]${NC} -> Add a new VLESS profile\n"
