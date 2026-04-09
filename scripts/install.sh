#!/usr/bin/env bash
# =============================================================================
#  Sambly Install / Uninstall Script
#  Supported: Debian 12, Ubuntu 22.04/24.04
#  Usage:
#    sudo bash install.sh              → install
#    sudo bash install.sh --uninstall  → remove Sambly
# =============================================================================

set -euo pipefail

# ── Source directory (resolved BEFORE any cd commands) ───────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(dirname "${SCRIPT_DIR}")"

# ── Colors ───────────────────────────────────────────────────────────────────
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
EOF
echo -e "${RESET}"

# ── Root check ───────────────────────────────────────────────────────────────
if [[ $EUID -ne 0 ]]; then
  die "This script must be run as root. Use: sudo bash install.sh"
fi

# ── Constants ─────────────────────────────────────────────────────────────────
INSTALL_DIR="/opt/sambly"
DATA_DIR="/var/lib/sambly"
LOG_DIR="/var/log/sambly"
SERVICE_USER="sambly"
BINARY_NAME="sambly"
GO_VERSION="1.22.3"
GO_ARCH="linux-amd64"
GO_TAR="go${GO_VERSION}.${GO_ARCH}.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"

# =============================================================================
#  UNINSTALL
# =============================================================================
if [[ "${1:-}" == "--uninstall" || "${1:-}" == "remove" || "${1:-}" == "uninstall" ]]; then
  echo -e "${YELLOW}${BOLD}╔══════════════════════════════════════════════════════╗${RESET}"
  echo -e "${YELLOW}${BOLD}║              Sambly Uninstaller                      ║${RESET}"
  echo -e "${YELLOW}${BOLD}╚══════════════════════════════════════════════════════╝${RESET}"
  echo

  # Ask about Samba service
  read -rp "$(echo -e "${YELLOW}Remove Samba service (smbd/nmbd)? [Y/n]: ${RESET}")" rm_samba
  rm_samba="${rm_samba:-Y}"

  # Ask about smb.conf
  read -rp "$(echo -e "${YELLOW}Remove Samba configuration (/etc/samba/smb.conf)? [Y/n]: ${RESET}")" rm_conf
  rm_conf="${rm_conf:-Y}"

  echo
  step "Stopping Sambly service..."
  systemctl stop sambly 2>/dev/null && ok "sambly stopped." || warn "sambly was not running."
  systemctl disable sambly 2>/dev/null || true
  rm -f /etc/systemd/system/sambly.service
  systemctl daemon-reload
  ok "sambly.service removed."

  step "Removing Sambly files..."
  rm -rf "${INSTALL_DIR}"
  rm -rf "${DATA_DIR}"
  rm -rf "${LOG_DIR}"
  rm -f /etc/sudoers.d/sambly
  ok "Sambly files removed."

  step "Removing service user '${SERVICE_USER}'..."
  # Remove from supplementary groups first (gpasswd removes membership cleanly)
  for grp in sambashare systemd-journal; do
    if getent group "${grp}" | grep -qw "${SERVICE_USER}"; then
      gpasswd -d "${SERVICE_USER}" "${grp}" 2>/dev/null && info "Removed from group: ${grp}" || true
    fi
  done
  # Delete the user (also removes it from all remaining group memberships)
  userdel "${SERVICE_USER}" 2>/dev/null && ok "User '${SERVICE_USER}' removed." || warn "User '${SERVICE_USER}' not found."

  if [[ "${rm_samba^^}" != "N" ]]; then
    step "Removing Samba service..."
    systemctl stop smbd nmbd 2>/dev/null || true
    systemctl disable smbd nmbd 2>/dev/null || true
    apt-get remove --purge -y samba samba-common smbclient 2>/dev/null || true
    ok "Samba removed."
  else
    info "Samba service kept."
  fi

  if [[ "${rm_conf^^}" != "N" ]]; then
    step "Removing smb.conf..."
    rm -f /etc/samba/smb.conf
    ok "smb.conf removed."
  else
    info "smb.conf kept."
  fi

  echo
  echo -e "${GREEN}${BOLD}Sambly has been successfully uninstalled.${RESET}"
  exit 0
fi

# =============================================================================
#  INSTALL
# =============================================================================

# ── OS Detection ─────────────────────────────────────────────────────────────
step "Checking prerequisites..."

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

# ── Port selection ────────────────────────────────────────────────────────────
echo
echo -e "${BOLD}Port Configuration${RESET}"
read -rp "$(echo -e "  Web GUI port [${GREEN}8090${RESET}]: ")" USER_PORT
PORT="${USER_PORT:-8090}"

# Validate port
if ! [[ "${PORT}" =~ ^[0-9]+$ ]] || [[ "${PORT}" -lt 1 || "${PORT}" -gt 65535 ]]; then
  die "Invalid port: ${PORT}. Must be 1-65535."
fi
info "Using port: ${PORT}"

# ── Install system dependencies ───────────────────────────────────────────────
step "Installing system dependencies..."

apt-get update -qq
apt-get install -y --no-install-recommends \
  samba smbclient sqlite3 curl ca-certificates \
  build-essential gcc 2>/dev/null || true

ok "System dependencies installed."

# ── Check/Install Go ──────────────────────────────────────────────────────────
step "Checking Go installation..."

if command -v go &>/dev/null; then
  CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
  info "Go ${CURRENT_GO} found at $(command -v go)"
else
  info "Go not found. Installing Go ${GO_VERSION}..."
  (
    cd /tmp
    curl -fsSL "${GO_URL}" -o "${GO_TAR}" || die "Failed to download Go"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "${GO_TAR}"
    rm -f "${GO_TAR}"
  )
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

usermod -aG sambashare "${SERVICE_USER}" 2>/dev/null || true
# Allow sambly to read systemd journal (for Samba log tab)
usermod -aG systemd-journal "${SERVICE_USER}" 2>/dev/null || true

# ── Build Sambly ──────────────────────────────────────────────────────────────
step "Building Sambly..."

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

ok "Binary built successfully."

# ── Install files ──────────────────────────────────────────────────────────────
step "Installing Sambly to ${INSTALL_DIR}..."

mkdir -p "${INSTALL_DIR}" "${DATA_DIR}" "${LOG_DIR}"
mkdir -p "${DATA_DIR}/backups"

if systemctl is-active --quiet sambly 2>/dev/null; then
  systemctl stop sambly
  info "Stopped existing sambly service."
fi

cp "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod 0755 "${INSTALL_DIR}/${BINARY_NAME}"

if [[ -d "${SRC_DIR}/web" ]]; then
  cp -r "${SRC_DIR}/web" "${INSTALL_DIR}/"
  ok "Web assets installed."
else
  warn "web/ directory not found — UI may not load correctly."
fi

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
   usershare allow guests = no

SMBCONF
  ok "smb.conf created."
else
  ok "smb.conf already exists."
fi

cp /etc/samba/smb.conf "${DATA_DIR}/backups/smb.conf.install-$(date +%Y%m%d-%H%M%S)"
ok "smb.conf backed up."

# Allow sambly user to write smb.conf directly (group ownership, no sudo needed)
chown root:"${SERVICE_USER}" /etc/samba/smb.conf
chmod 664 /etc/samba/smb.conf
ok "smb.conf ownership set to root:${SERVICE_USER} (664)"

# ── Systemd service ────────────────────────────────────────────────────────────
step "Installing systemd service..."

# Detect real systemctl path (Debian 12 uses /usr/bin/systemctl)
SYSTEMCTL_BIN="$(command -v systemctl 2>/dev/null || echo /usr/bin/systemctl)"
info "systemctl path: ${SYSTEMCTL_BIN}"

cat > /etc/sudoers.d/sambly <<SUDOERS
# Sambly service management — generated by install.sh
sambly ALL=(ALL) NOPASSWD: ${SYSTEMCTL_BIN} start smbd
sambly ALL=(ALL) NOPASSWD: ${SYSTEMCTL_BIN} stop smbd
sambly ALL=(ALL) NOPASSWD: ${SYSTEMCTL_BIN} restart smbd
sambly ALL=(ALL) NOPASSWD: ${SYSTEMCTL_BIN} reload smbd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/useradd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/userdel
sambly ALL=(ALL) NOPASSWD: /usr/sbin/usermod
sambly ALL=(ALL) NOPASSWD: /usr/sbin/groupadd
sambly ALL=(ALL) NOPASSWD: /usr/sbin/groupdel
sambly ALL=(ALL) NOPASSWD: /usr/bin/smbpasswd
sambly ALL=(ALL) NOPASSWD: /usr/bin/pdbedit
sambly ALL=(ALL) NOPASSWD: /usr/bin/net
sambly ALL=(ALL) NOPASSWD: /usr/bin/gpasswd
sambly ALL=(ALL) NOPASSWD: /bin/mkdir
sambly ALL=(ALL) NOPASSWD: /usr/bin/mkdir
sambly ALL=(ALL) NOPASSWD: /bin/chown
sambly ALL=(ALL) NOPASSWD: /usr/bin/chown
sambly ALL=(ALL) NOPASSWD: /bin/chmod
sambly ALL=(ALL) NOPASSWD: /usr/bin/chmod
SUDOERS
chmod 0440 /etc/sudoers.d/sambly

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
  -addr 0.0.0.0:${PORT} \\
  -data ${DATA_DIR} \\
  -web ${INSTALL_DIR}/web
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sambly
PrivateTmp=yes
ProtectSystem=yes
ProtectHome=yes

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

# ── Start Sambly & read credentials ───────────────────────────────────────────
step "Starting Sambly service..."

systemctl start sambly || die "Failed to start sambly — check 'journalctl -u sambly -n 30'"
sleep 3  # wait for first-run credential generation

# Read credentials file written by the binary on first start
CRED_FILE="${DATA_DIR}/initial-credentials.txt"
ADMIN_USER="admin"
ADMIN_PASS=""

if [[ -f "${CRED_FILE}" ]]; then
  ADMIN_PASS=$(grep "^PASSWORD=" "${CRED_FILE}" | cut -d= -f2)
fi

if systemctl is-active --quiet sambly; then
  ok "Sambly is running."
else
  warn "Sambly may not have started. Check: journalctl -u sambly -n 30"
fi

# ── Final output ───────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════════════╗${RESET}"
echo -e "${GREEN}${BOLD}║         Sambly Installation Complete!                ║${RESET}"
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  URL:      ${BOLD}http://localhost:${PORT}${RESET}                       ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Service:  sambly.service (systemd)                  ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Data:     ${DATA_DIR}                       ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  ${BOLD}Admin Login${RESET}                                          ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  Username: ${BOLD}${ADMIN_USER}${RESET}                                     ${GREEN}${BOLD}║${RESET}"
if [[ -n "${ADMIN_PASS}" ]]; then
  printf "${GREEN}${BOLD}║${RESET}  Password: ${BOLD}%-42s${GREEN}${BOLD}║${RESET}\n" "${ADMIN_PASS}"
else
  echo -e "${GREEN}${BOLD}║${RESET}  Password: $(journalctl -u sambly -n 80 --no-pager 2>/dev/null | grep "^PASSWORD=" | cut -d= -f2 || echo '(run: journalctl -u sambly -n 50)')  ${GREEN}${BOLD}║${RESET}"
fi
echo -e "${GREEN}${BOLD}╠══════════════════════════════════════════════════════╣${RESET}"
echo -e "${GREEN}${BOLD}║${RESET}  ${YELLOW}${BOLD}⚠  Change your password after first login!${RESET}         ${GREEN}${BOLD}║${RESET}"
echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════════════╝${RESET}"
echo ""
info "Manage service:  systemctl {start|stop|restart|status} sambly"
info "View logs:       journalctl -u sambly -f"
info "Uninstall:       sudo bash scripts/install.sh --uninstall"
