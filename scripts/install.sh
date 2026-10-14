#!/usr/bin/env bash
set -e

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}==============================================${NC}"
echo -e "${GREEN}   VLESS Linux System-Wide TUN Bypass Tool    ${NC}"
echo -e "${BLUE}==============================================${NC}"

# Check root privileges
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}[-] Please run this installer as root: sudo ./install.sh${NC}"
  exit 1
fi

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

echo -e "${YELLOW}[*] Detected architecture: $GOARCH${NC}"

# Ensure /dev/net/tun exists
if [ ! -c /dev/net/tun ]; then
  echo -e "${YELLOW}[*] Creating /dev/net/tun device...${NC}"
  mkdir -p /dev/net
  mknod /dev/net/tun c 10 200
  chmod 666 /dev/net/tun
fi

# Create system config directory
mkdir -p /etc/vless-linux
chmod 755 /etc/vless-linux

# Install sing-box core binary
SINGBOX_VERSION="1.8.14"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -f "/usr/local/bin/sing-box" ]; then
  echo -e "${GREEN}[+] Sing-box core already present at /usr/local/bin/sing-box${NC}"
elif [ -f "/root/.vless-linux/bin/sing-box" ]; then
  echo -e "${YELLOW}[*] Installing Sing-box core from local cache...${NC}"
  cp /root/.vless-linux/bin/sing-box /usr/local/bin/sing-box
  chmod +x /usr/local/bin/sing-box
elif [ -f "$SCRIPT_DIR/sing-box" ]; then
  cp "$SCRIPT_DIR/sing-box" /usr/local/bin/sing-box
  chmod +x /usr/local/bin/sing-box
else
  echo -e "${YELLOW}[*] Downloading Sing-box core v${SINGBOX_VERSION}...${NC}"
  TMP_DIR=$(mktemp -d)
  SINGBOX_URL="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}.tar.gz"
  curl -fsSL "$SINGBOX_URL" -o "$TMP_DIR/sing-box.tar.gz" || true
  if [ -f "$TMP_DIR/sing-box.tar.gz" ]; then
    tar -xzf "$TMP_DIR/sing-box.tar.gz" -C "$TMP_DIR"
    cp "$TMP_DIR"/sing-box-*/sing-box /usr/local/bin/sing-box
    chmod +x /usr/local/bin/sing-box
  fi
  rm -rf "$TMP_DIR"
fi

if [ -f "/usr/local/bin/sing-box" ]; then
  setcap cap_net_admin,cap_net_bind_service+ep /usr/local/bin/sing-box || true
fi

# Install vless-linux binary to /usr/local/bin
if [ -f "$SCRIPT_DIR/vless-linux" ]; then
  echo -e "${YELLOW}[*] Installing vless-linux binary to /usr/local/bin...${NC}"
  cp "$SCRIPT_DIR/vless-linux" /usr/local/bin/vless-linux
  chmod +x /usr/local/bin/vless-linux
elif command -v go &> /dev/null; then
  echo -e "${YELLOW}[*] Compiling vless-linux with Go...${NC}"
  cd "$SCRIPT_DIR"
  go build -ldflags="-s -w" -o /usr/local/bin/vless-linux ./cmd/vless-linux
  chmod +x /usr/local/bin/vless-linux
else
  echo -e "${RED}[-] Error: vless-linux binary not found in $SCRIPT_DIR${NC}"
  exit 1
fi

setcap cap_net_admin,cap_net_bind_service+ep /usr/local/bin/vless-linux || true

# Setup systemd service template
echo -e "${YELLOW}[*] Installing systemd service template...${NC}"
cat << 'EOF' > /etc/systemd/system/vless-linux.service
[Unit]
Description=VLESS System-Wide TUN Bypass Service
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vless-linux connect
Restart=always
RestartSec=3s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

echo -e "${GREEN}==============================================${NC}"
echo -e "${GREEN}[+] Tool Installed Globally to /usr/local/bin!${NC}"
echo -e "${GREEN}==============================================${NC}"
echo -e "${CYAN}[i] You can now safely delete the source/setup folder if you wish.${NC}\n"
echo -e "Usage:"
echo -e "  ${BLUE}sudo vless-linux${NC}                       -> Launch Interactive TUI Dashboard"
echo -e "  ${BLUE}sudo vless-linux connect${NC}               -> Connect in background (terminal close won't drop)"
echo -e "  ${BLUE}sudo vless-linux disconnect${NC}            -> Disconnect cleanly"
echo -e "  ${BLUE}vless-linux status${NC}                     -> Check live speed and status"
echo -e "  ${BLUE}vless-linux profile add \"<url>\" [name]${NC}  -> Add a new VLESS profile\n"
