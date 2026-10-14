#!/usr/bin/env bash
set -e

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Install dependencies if missing
echo -e "${YELLOW}[*] Checking required system packages...${NC}"
if command -v apt-get &> /dev/null; then
  apt-get update -qq
  apt-get install -y -qq curl tar iproute2 iptables > /dev/null
elif command -v pacman &> /dev/null; then
  pacman -Sy --noconfirm curl tar iproute2 iptables > /dev/null
elif command -v dnf &> /dev/null; then
  dnf install -y -q curl tar iproute iptables > /dev/null
fi

# Ensure /dev/net/tun exists
if [ ! -c /dev/net/tun ]; then
  echo -e "${YELLOW}[*] Creating /dev/net/tun device...${NC}"
  mkdir -p /dev/net
  mknod /dev/net/tun c 10 200
  chmod 666 /dev/net/tun
fi

# Install sing-box core
SINGBOX_VERSION="1.8.14"
SINGBOX_URL="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${GOARCH}.tar.gz"

echo -e "${YELLOW}[*] Installing Sing-box core v${SINGBOX_VERSION}...${NC}"
TMP_DIR=$(mktemp -d)
curl -fsSL "$SINGBOX_URL" -o "$TMP_DIR/sing-box.tar.gz"
tar -xzf "$TMP_DIR/sing-box.tar.gz" -C "$TMP_DIR"
cp "$TMP_DIR"/sing-box-*/sing-box /usr/local/bin/sing-box
chmod +x /usr/local/bin/sing-box
rm -rf "$TMP_DIR"

# Allow sing-box to bind TUN interfaces without root
setcap cap_net_admin,cap_net_bind_service+ep /usr/local/bin/sing-box || true

# If precompiled vless-linux binary exists in directory, install it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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
  echo -e "${YELLOW}[!] Note: vless-linux binary will need to be placed at /usr/local/bin/vless-linux${NC}"
fi

# Setup systemd service file
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
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

echo -e "${GREEN}==============================================${NC}"
echo -e "${GREEN}[+] Installation Completed Successfully!      ${NC}"
echo -e "${GREEN}==============================================${NC}"
echo -e "To connect your system, run:"
echo -e "  ${BLUE}sudo vless-linux connect \"your_vless_link\"${NC}\n"
echo -e "To auto-connect on system boot via systemd:"
echo -e "  ${BLUE}sudo systemctl enable --now vless-linux${NC}\n"
