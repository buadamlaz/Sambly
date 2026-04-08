#!/usr/bin/env bash
# =============================================================================
#  Sambly Install Script
#  Supported: Debian 12, Ubuntu 22.04/24.04
#  Run as: sudo bash install.sh
# =============================================================================

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { echo -e "${BLUE}[INFO]${RESET}  $*"; }
ok()    { echo -e "${GREEN}[OK]${RESET}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
error() { echo -e "${RED}[ERROR]${RESET} $*" >&2; }
step()  { echo -e "\n${BOLD}[STEP]${RESET}  $*"; }
die()   { error "$*"; exit 1; }

# ── Banner ───────────────────────────────────────────────────────────────────
echo -e "${BOLD}"
cat <<'EOF'
 ____                 _     _
/ ___|  __ _ _ __ ___ | |__ | |_   _
\___ \ / _' | '_ ' _ \| '_ \| | | | |
 ___) | (_| | | | | | | |_) | | |_| |
|____/ \__,_|_| |_| |_|_.__/|_|\__, |
                                 |___/
  Samba management, simplified.
  Install Script — v1.0.0
EOF
echo -e "${RESET}"

# ── Checks ───────────────────────────────────────────────────────────────────
step "Checking prerequisites..."

if [[ $EUID -ne 0 ]]; then
  die "This script must be run as root. Use: sudo bash install.sh"
fi

# Detect OS
if [[ -f /etc/os-release ]]; then
  source /etc/os-release
  OS_ID="${ID:-unknown}"
  OS_PRETTY="${PRETTY_NAME:-unknown}"
  info "Detected: ${OS_PRETTY}"
else
  die "Cannot detect OS. /etc/os-release not found."
fi

case "${OS_ID}" in
  debian|ubuntu|raspbian|linuxmint) ;;
  *) warn "OS '${OS_ID}' not officially tested. Proceeding anyway...";;
esac

# ── Variables ─────────────────────────────────────────────────────────────────
INSTALL_DIR="/opt/sambly"
DATA_DIR="/var/lib/sambly"
LOG_DIR="/var/log/sambly"
SERVICE_USER="sambly"
BINARY_NAME="sambly"
GO_VERSION="1.22.3"
GO_ARCH="linux-amd64"
GO_TAR="go${GO_VERSION}.${GO_ARCH}.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"

# ── Install system dependencies ───────────────────────────────────────────────
step "Installing system dependencies..."

apt-get update -qq
apt-get install -y --no-install-recommends \
  samba smbclient sqlite3 curl ca-certificates \
  build-essential gcc \
  systemd 2>/dev/null || true

ok "System dependencies installed."

# ── Check/Install Go ──────────────────────────────────────────────────────────
step "Checking Go installation..."

if command -v go &>/dev/null; then
  CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
  info "Go ${CURRENT_GO} found at $(command -v go)"
else
  info "Go not found. Installing Go ${GO_VERSION}..."
  cd /tmp
  curl -fsSL "${GO_URL}" -o "${GO_TAR}" || die "Failed to download Go"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "${GO_TAR}"
  rm -f "${GO_TAR}"
  export PATH="/usr/local/go/bin:${PATH}"
  ok "Go ${GO_VERSION} installed."
fi

export PATH="/usr/local/go/bin:${PATH}"

if ! command -v go &>/dev/null; then
  die "Go installation failed."
fi
ok "Go $(go version) ready."

# ── Create service user ────────────────────────────────────────────────────────
step "Creating service user '${SERVICE_USER}'..."

if id "${SERVICE_USER}" &>/dev/null; then
  info "User '${SERVICE_USER}' already exists."
else
  useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
  ok "User '${SERVICE_USER}' created."
fi

# Add sambly to required groups
usermod -aG sambashare "${SERVICE_USER}" 2>/dev/null || true

# ── Build Sambly ──────────────────────────────────────────────────────────────
step "Building Sambly..."

# Determine source directory (script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(dirname "${SCRIPT_DIR}")"

if [[ ! -f "${SRC_DIR}/go.mod" ]]; then
  die "go.mod not found in ${SRC_DIR}. Run this script from the Sambly source directory."
fi

cd "${SRC_DIR}"

info "Downloading Go dependencies..."
go mod download || die "go mod download failed"

info "Building binary..."
CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o "/tmp/${BINARY_NAME}" \
  ./cmd/server || die "Build failed"

ok "Binary built: /tmp/${BINARY_NAME}"

# ── Install files ──────────────────────────────────────────────────────────────
step "Installing Sambly to ${INSTALL_DIR}..."

mkdir -p "${INSTALL_DIR}" "${DATA_DIR}" "${LOG_DIR}"
mkdir -p "${DATA_DIR}/backups"

# Stop existing service if running
if systemctl is-active --quiet sambly 2>/dev/null; then
  systemctl stop sambly
  info "Stopped existing sambly service."
fi

# Copy binary
cp "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"

# Copy web assets
if [[ -d "${SRC_DIR}/web" ]]; then
  cp -r "${SRC_DIR}/web" "${INSTALL_DIR}/"
  ok "Web assets installed."
else
  warn "web/ directory not found — UI may not load correctly."
fi

# Set ownership
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${INSTALL_DIR}"
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${DATA_DIR}"
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${LOG_DIR}"

ok "Sambly installed to ${INSTALL_DIR}."

# ── Samba configuration ────────────────────────────────────────────────────────
step "Checking Samba configuration..."

if [[ ! -f /etc/samba/smb.conf ]]; then
  warn "smb.conf not found. Creating minimal config..."
  cat > /etc/samba/smb.conf <<'SMBCONF'
[global]
   workgroup = WORKGROUP
   server string = Samba Server %v
   netbios name = SAMBASERVER
   security = user
   map to guest = never
   dns proxy = no
   log file = /var/log/samba/log.%m
   max log size = 1000
   logging = file
   panic action = /usr/share/samba/panic-action %d
   server role = standalone server
   passdb backend = tdbsam
   obey pam restrictions = yes
   unix password sync = no
   pam password change = yes
   passwd program = /usr/bin/passwd %u
   passwd chat = *Enter\snew\s*\spassword:* %n\n *Retype\snew\s*\spassword:* %n\n *password\supdated\ssuccessfully* .
   usershare allow guests = no

SMBCONF
  ok "smb.conf created."
else
  ok "smb.conf already exists."
fi

# Backup smb.conf
cp /etc/samba/smb.conf "${DATA_DIR}/backups/smb.conf.install-$(date +%Y%m%d-%H%M%S)"
ok "smb.conf backed up."

# ── Systemd service ────────────────────────────────────────────────────────────
step "Installing systemd service..."

# Grant sambly user ability to manage smbd via sudoers
cat > /etc/sudoers.d/sambly <<'SUDOERS'
# Sambly service management
sambly ALL=(ALL) NOPASSWD: /bin/systemctl start smbd
sambly ALL=(ALL) NOPASSWD: /bin/systemctl stop smbd
sambly ALL=(ALL) NOPASSWD: /bin/systemctl restart smbd
sambly ALL=(ALL) NOPASSWD: /bin/systemctl reload smbd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/useradd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/userdel
sambly ALL=(ALL) NOPASSWD: /usr/sbin/usermod
sambly ALL=(ALL) NOPASSWD: /usr/sbin/groupadd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/groupdel
sambly ALL=(ALL) NOPASSWD: /usr/bin/smbpasswd
sambly ALL=(ALL) NOPASSWD: /usr/bin/pdbedit
sambly ALL=(ALL) NOPASSWD: /usr/bin/net
sambly ALL=(ALL) NOPASSWD: /usr/bin/gpasswd
SUDOERS
chmod 0440 /etc/sudoers.d/sambly

# Install service file
cat > /etc/systemd/system/sambly.service <<UNIT
[Unit]
Description=Sambly — Samba Web Management GUI
Documentation=https://github.com/buadamlaz/Sambly
After=network.target samba.service
Wants=network.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME} \\
  -addr 127.0.0.1:8090 \\
  -data ${DATA_DIR} \\
  -web ${INSTALL_DIR}/web
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sambly

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=${DATA_DIR} /etc/samba ${LOG_DIR}
ProtectHome=yes
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable sambly
ok "sambly.service installed and enabled."

# ── Start Samba ────────────────────────────────────────────────────────────────
step "Starting Samba (smbd)..."

systemctl enable smbd nmbd 2>/dev/null || true
systemctl start smbd || warn "Failed to start smbd — check 'systemctl status smbd'"
ok "Samba started."

# ── Start Sambly ──────────────────────────────────────────────────────────────
step "Starting Sambly service..."

systemctl start sambly || die "Failed to start sambly — check 'journalctl -u sambly -n 50'"
sleep 2

if systemctl is-active --quiet sambly; then
  ok "Sambly is running."
else
  warn "Sambly may not have started. Check: journalctl -u sambly -n 30"
fi

# ── Get credentials from journal ──────────────────────────────────────────────
sleep 1
CREDS=$(journalctl -u sambly --since "1 minute ago" --no-pager 2>/dev/null | grep -A5 "SAMBLY" | head -20 || true)

# ── Final output ───────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════════════╗${RESET}"
echo -e "${GREEN}${BOLD}║         Sambly Installation Complete!                ║${RESET}"
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  URL:      ${BOLD}http://127.0.0.1:8090${RESET}                    ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Service:  sambly.service (systemd)              ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Data:     ${DATA_DIR}                    ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Logs:     journalctl -u sambly -f               ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  ${YELLOW}${BOLD}Credentials are shown at first startup.${RESET}         ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Run: ${BOLD}journalctl -u sambly -n 50${RESET}                  ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  ${RED}⚠  DO NOT expose Sambly to the internet!${RESET}       ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════════════╝${RESET}"
echo ""

info "View credentials: journalctl -u sambly --no-pager | grep -A6 'CREDENTIALS'"
info "Manage service:   systemctl {start|stop|restart|status} sambly"
info "View logs:        journalctl -u sambly -f"
