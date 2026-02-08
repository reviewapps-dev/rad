#!/usr/bin/env bash
set -euo pipefail

# ReviewApps.dev — rad installer for Ubuntu
#
# Usage:
#   sudo bash install.sh --token=secret
#   sudo bash install.sh --token=secret --version=0.0.1
#
# For testing without a ReviewApps.dev backend:
#   sudo bash install.sh --token=secret
#   curl http://localhost:7890/health
#   curl -H "Authorization: Bearer secret" http://localhost:7890/apps

RAD_VERSION="${RAD_VERSION:-latest}"
RAD_REPO="reviewapps-dev/rad"
INSTALL_DIR="/opt/reviewapps"

# Parse arguments
TOKEN=""
API_ENDPOINT=""

for arg in "$@"; do
  case "$arg" in
    --token=*) TOKEN="${arg#*=}" ;;
    --api-endpoint=*) API_ENDPOINT="${arg#*=}" ;;
    --version=*) RAD_VERSION="${arg#*=}" ;;
    --help|-h)
      echo "Usage: sudo bash install.sh --token=<token> [--version=0.0.1] [--api-endpoint=http://...]"
      exit 0
      ;;
    *) echo "Unknown argument: $arg"; exit 1 ;;
  esac
done

if [ -z "$TOKEN" ]; then
  echo "Error: --token is required"
  echo "Usage: sudo bash install.sh --token=secret"
  exit 1
fi

# --- Helpers ---

info()  { echo -e "\033[1;34m==>\033[0m \033[1m$*\033[0m"; }
ok()    { echo -e "\033[1;32m  ✓\033[0m $*"; }
warn()  { echo -e "\033[1;33m  !\033[0m $*"; }
fail()  { echo -e "\033[1;31m  ✗\033[0m $*"; exit 1; }

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    fail "This script must be run as root (try: sudo bash install.sh --token=...)"
  fi
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) fail "Unsupported architecture: $arch" ;;
  esac
}

# --- Main ---

require_root

ARCH="$(detect_arch)"

info "Installing ReviewApps.dev daemon (rad)"
echo "  Arch:    linux/$ARCH"
echo "  Version: $RAD_VERSION"
echo ""

# 1. System dependencies
info "Installing system dependencies..."
export DEBIAN_FRONTEND=noninteractive

PACKAGES=(
  build-essential
  libpq-dev
  libsqlite3-dev
  libvips-dev
  libffi-dev
  libssl-dev
  libyaml-dev
  git
  curl
  unzip
  postgresql-client
)

apt-get update -qq
for pkg in "${PACKAGES[@]}"; do
  printf "  installing %-25s" "$pkg..."
  apt-get install -y -qq "$pkg" > /dev/null 2>&1 && echo " done" || echo " failed"
done
ok "System dependencies installed"

# 2. Create directory structure
info "Creating directory structure..."
mkdir -p "$INSTALL_DIR"/{bin,etc/caddy/sites,apps,log,tmp}
ok "$INSTALL_DIR/{bin,etc,apps,log,tmp}"

# 3. Download rad
info "Downloading rad..."
if [ "$RAD_VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/$RAD_REPO/releases/latest/download/rad_linux_${ARCH}.tar.gz"
else
  DOWNLOAD_URL="https://github.com/$RAD_REPO/releases/download/v${RAD_VERSION}/rad_linux_${ARCH}.tar.gz"
fi

TMP_DIR="$(mktemp -d)"
curl -sSL "$DOWNLOAD_URL" -o "$TMP_DIR/rad.tar.gz" || fail "Failed to download rad from $DOWNLOAD_URL"
tar -xzf "$TMP_DIR/rad.tar.gz" -C "$TMP_DIR"
cp "$TMP_DIR"/rad_*/rad "$INSTALL_DIR/bin/rad"
chmod +x "$INSTALL_DIR/bin/rad"
rm -rf "$TMP_DIR"

RAD_INSTALLED_VERSION="$($INSTALL_DIR/bin/rad version 2>&1 | awk '{print $2}')"
ok "rad $RAD_INSTALLED_VERSION"

# 4. Install rv (Ruby version manager)
info "Installing rv..."
if [ ! -f "$INSTALL_DIR/bin/rv" ]; then
  RV_VERSION="0.4.3"
  RV_URL="https://github.com/nicholasgasior/rv/releases/download/v${RV_VERSION}/rv-${RV_VERSION}-linux-${ARCH}"
  curl -sSL "$RV_URL" -o "$INSTALL_DIR/bin/rv" || fail "Failed to download rv"
  chmod +x "$INSTALL_DIR/bin/rv"
  ok "rv $RV_VERSION"
else
  ok "rv already installed"
fi

# 5. Install fnm (Fast Node Manager)
info "Installing fnm..."
if [ ! -f "$INSTALL_DIR/bin/fnm" ]; then
  curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$INSTALL_DIR/bin" --skip-shell > /dev/null 2>&1
  ok "fnm installed"
else
  ok "fnm already installed"
fi

# 6. Install Caddy
info "Installing Caddy..."
if ! command -v caddy &> /dev/null; then
  apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https > /dev/null 2>&1
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' 2>/dev/null | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg 2>/dev/null
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' 2>/dev/null | tee /etc/apt/sources.list.d/caddy-stable.list > /dev/null
  apt-get update -qq
  apt-get install -y -qq caddy > /dev/null 2>&1
  ok "Caddy $(caddy version 2>/dev/null | awk '{print $1}')"
else
  ok "Caddy already installed ($(caddy version 2>/dev/null | awk '{print $1}'))"
fi

# 7. Caddy config
info "Configuring Caddy..."
cat > "$INSTALL_DIR/etc/caddy/Caddyfile" <<'CADDYFILE'
{
  admin off
}

import /opt/reviewapps/etc/caddy/sites/*.caddy
CADDYFILE
ok "Caddyfile written"

# 8. rad config
info "Writing rad config..."
SERVER_ID="srv_$(head -c 8 /dev/urandom | xxd -p)"

cat > "$INSTALL_DIR/etc/config.toml" <<TOML
[server]
listen = "0.0.0.0:7890"

[api]
server_id = "$SERVER_ID"
$([ -n "$API_ENDPOINT" ] && echo "endpoint = \"$API_ENDPOINT\"" || echo "# endpoint = \"https://reviewapps.dev/api/v1\"  # no backend configured")
$([ -n "$API_ENDPOINT" ] && echo "api_key = \"$TOKEN\"" || echo "# api_key = \"$TOKEN\"")

[paths]
apps_dir = "$INSTALL_DIR/apps"
log_dir = "$INSTALL_DIR/log"

[caddy]
enabled = true
config_dir = "$INSTALL_DIR/etc/caddy/sites"

[defaults]
ruby_version = "3.4.1"
database_adapter = "sqlite"
TOML
ok "Config: $INSTALL_DIR/etc/config.toml"

# 9. systemd service
info "Creating systemd service..."
cat > /etc/systemd/system/rad.service <<SERVICE
[Unit]
Description=ReviewApps.dev Daemon
After=network.target
Wants=caddy.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/rad --config $INSTALL_DIR/etc/config.toml --token $TOKEN
Restart=always
RestartSec=5
LimitNOFILE=65535
LimitNPROC=4096

Environment=HOME=/root
Environment=PATH=$INSTALL_DIR/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

StandardOutput=append:$INSTALL_DIR/log/rad.log
StandardError=append:$INSTALL_DIR/log/rad.log

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable rad > /dev/null 2>&1
ok "systemd service created"

# 10. Start
info "Starting services..."
systemctl stop caddy 2>/dev/null || true
# Point Caddy at our Caddyfile instead of the default
systemctl start caddy 2>/dev/null || warn "Caddy failed to start (check: systemctl status caddy)"
systemctl start rad
ok "rad started"

# 11. Verify
info "Verifying..."
sleep 2
if curl -sf http://localhost:7890/health > /dev/null 2>&1; then
  HEALTH="$(curl -sf http://localhost:7890/health)"
  ok "rad is healthy"
  echo "     $HEALTH"
else
  warn "rad may still be starting — check: tail -f $INSTALL_DIR/log/rad.log"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ReviewApps.dev daemon installed!"
echo ""
echo "  Token:     $TOKEN"
echo "  Server ID: $SERVER_ID"
echo "  Config:    $INSTALL_DIR/etc/config.toml"
echo "  Logs:      $INSTALL_DIR/log/rad.log"
echo ""
echo "  Test it:"
echo "    curl http://localhost:7890/health"
echo "    curl -H 'Authorization: Bearer $TOKEN' http://localhost:7890/apps"
echo ""
echo "  Deploy a test app:"
echo "    curl -X POST http://localhost:7890/apps/deploy \\"
echo "      -H 'Authorization: Bearer $TOKEN' \\"
echo "      -H 'Content-Type: application/json' \\"
echo "      -d '{\"app_id\":\"test-app\",\"repo_url\":\"https://github.com/owner/repo.git\",\"branch\":\"main\",\"subdomain\":\"test-app\"}'"
echo ""
echo "  Manage:"
echo "    sudo systemctl status rad"
echo "    sudo systemctl restart rad"
echo "    tail -f $INSTALL_DIR/log/rad.log"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
